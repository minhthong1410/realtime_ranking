package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"realtime_ranking/pkg/httputil"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func setupTestHandler(t *testing.T) (*RankingHandler, *miniredis.Miniredis, func()) {
	// Start miniredis
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}

	// Create Redis client
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	// Create logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	// Create handler
	handler := &RankingHandler{
		redis:  client,
		logger: logger,
	}

	// Cleanup function
	cleanup := func() {
		client.Close()
		mr.Close()
	}

	return handler, mr, cleanup
}

func TestUpdateScore(t *testing.T) {
	handler, mr, cleanup := setupTestHandler(t)
	defer cleanup()

	ctx := context.Background()

	// Initialize a video
	video := Video{
		ID:        "vid1",
		Title:     "Test Video",
		CreatorID: "creator1",
		Score:     0.0,
	}
	err := handler.InitializeVideo(ctx, video)
	if err != nil {
		t.Fatalf("failed to initialize video: %v", err)
	}

	tests := []struct {
		name           string
		interaction    Interaction
		body           string
		expectedStatus int
		expectedScore  float64
	}{
		{
			name: "Valid view interaction",
			interaction: Interaction{
				VideoID:   "vid1",
				Type:      InteractionView,
				UserID:    "user1",
				Timestamp: 1234567890,
			},
			body:           `{"video_id":"vid1","type":"view","user_id":"user1","timestamp":1234567890}`,
			expectedStatus: http.StatusOK,
			expectedScore:  1.0,
		},
		{
			name: "Missing video auto-initialized",
			interaction: Interaction{
				VideoID:   "vid2",
				Type:      InteractionLike,
				UserID:    "user2",
				Timestamp: 1234567890,
			},
			body:           `{"video_id":"vid2","type":"like","user_id":"user2","timestamp":1234567890}`,
			expectedStatus: http.StatusOK,
			expectedScore:  5.0,
		},
		{
			name: "Invalid interaction type",
			interaction: Interaction{
				VideoID:   "vid1",
				Type:      "invalid",
				UserID:    "user1",
				Timestamp: 1234567890,
			},
			body:           `{"video_id":"vid1","type":"invalid","user_id":"user1","timestamp":1234567890}`,
			expectedStatus: http.StatusBadRequest,
			expectedScore:  0.0,
		},
		{
			name: "Missing user_id",
			interaction: Interaction{
				VideoID:   "vid1",
				Type:      InteractionView,
				UserID:    "",
				Timestamp: 1234567890,
			},
			body:           `{"video_id":"vid1","type":"view","user_id":"","timestamp":1234567890}`,
			expectedStatus: http.StatusBadRequest,
			expectedScore:  0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("POST", "/api/v1/interaction", bytes.NewReader([]byte(tt.body)))
			rr := httptest.NewRecorder()

			err := handler.UpdateScore(rr, req)
			if (err != nil) != (tt.expectedStatus != http.StatusOK) {
				t.Errorf("UpdateScore error = %v, want status %d", err, tt.expectedStatus)
			}

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			if tt.expectedStatus == http.StatusOK {
				var resp httputil.HttpResponse
				if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
					t.Errorf("failed to decode response: %v", err)
				}
				newScore, ok := resp.Data.(map[string]interface{})["new_score"].(float64)
				if !ok || newScore != tt.expectedScore {
					t.Errorf("expected new_score %f, got %v", tt.expectedScore, newScore)
				}

				// Verify Redis state
				score, err := handler.redis.ZScore(ctx, "rankings:global", tt.interaction.VideoID).Result()
				if err != nil || score != tt.expectedScore {
					t.Errorf("expected global score %f, got %f", tt.expectedScore, score)
				}

				if tt.name == "Missing video auto-initialized" {
					videoData, _ := handler.redis.HGetAll(ctx, "video:vid2").Result()
					if !strings.HasPrefix(videoData["title"], "Untitled Video") {
						t.Errorf("expected placeholder title, got %s", videoData["title"])
					}
					if videoData["creator_id"] != "user2" {
						t.Errorf("expected creator_id user2, got %s", videoData["creator_id"])
					}
				}
			})
		})
	}
}

func TestGetRanking(t *testing.T) {
	handler, mr, cleanup := setupTestHandler(t)
	defer cleanup()

	ctx := context.Background()

	// Initialize videos
	videos := []Video{
		{ID: "vid1", Title: "Video 1", CreatorID: "creator1", Score: 100.0},
		{ID: "vid2", Title: "Video 2", CreatorID: "creator2", Score: 50.0},
		{ID: "vid3", Title: "", CreatorID: "", Score: 0.0}, // Invalid video
	}
	for _, v := range videos {
		if v.Title != "" {
			err := handler.InitializeVideo(ctx, v)
			if err != nil {
				t.Fatalf("failed to initialize video %s: %v", v.ID, err)
			}
		} else {
			// Simulate invalid video (only in sorted set)
			mr.ZAdd("rankings:global", 25.0, v.ID)
		}
	}

	tests := []struct {
		name           string
		query          string
		expectedStatus int
		expectedVideos int
		expectedFirst  string
	}{
		{
			name:           "Default limit and offset",
			query:          "",
			expectedStatus: http.StatusOK,
			expectedVideos: 2,
			expectedFirst:  "vid1",
		},
		{
			name:           "Custom limit and offset",
			query:          "limit=1&offset=1",
			expectedStatus: http.StatusOK,
			expectedVideos: 1,
			expectedFirst:  "vid2",
		},
		{
			name:           "Invalid limit",
			query:          "limit=101",
			expectedStatus: http.StatusBadRequest,
			expectedVideos: 0,
		},
		{
			name:           "Negative offset",
			query:          "offset=-1",
			expectedStatus: http.StatusBadRequest,
			expectedVideos: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/api/v1/ranking?"+tt.query, nil)
			rr := httptest.NewRecorder()

			err := handler.GetRanking(rr, req)
			if (err != nil) != (tt.expectedStatus != http.StatusOK) {
				t.Errorf("GetRanking error = %v, want status %d", err, tt.expectedStatus)
			}

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			if tt.expectedStatus == http.StatusOK {
				var resp httputil.HttpResponse
				if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
					t.Errorf("failed to decode response: %v", err)
				}
				videos, ok := resp.Data.([]interface{})
				if !ok || len(videos) != tt.expectedVideos {
					t.Errorf("expected %d videos, got %d", tt.expectedVideos, len(videos))
				}
				if tt.expectedVideos > 0 {
					firstVideo := videos[0].(map[string]interface{})
					if firstVideo["id"] != tt.expectedFirst {
						t.Errorf("expected first video %s, got %s", tt.expectedFirst, firstVideo["id"])
					}
				}
			})
		})
	}
}

func TestGetPersonalRanking(t *testing.T) {
	handler, mr, cleanup := setupTestHandler(t)
	defer cleanup()

	ctx := context.Background()

	// Initialize videos
	videos := []Video{
		{ID: "vid1", Title: "Video 1", CreatorID: "creator1", Score: 50.0},
		{ID: "vid2", Title: "Video 2", CreatorID: "creator2", Score: 100.0},
		{ID: "vid3", Title: "Video 3", CreatorID: "creator3", Score: 25.0},
	}
	for _, v := range videos {
		err := handler.InitializeVideo(ctx, v)
		if err != nil {
			t.Fatalf("failed to initialize video %s: %v", v.ID, err)
		}
	}

	// Initialize user follows
	mr.SAdd("user:user1:follows", "creator1", "creator2")

	tests := []struct {
		name           string
		query          string
		expectedStatus int
		expectedVideos int
		expectedFirst  string
	}{
		{
			name:           "Valid user with follows",
			query:          "user_id=user1&limit=2",
			expectedStatus: http.StatusOK,
			expectedVideos: 2,
			expectedFirst:  "vid1", // creator1, score 50+100=150
		},
		{
			name:           "Missing user_id",
			query:          "limit=5",
			expectedStatus: http.StatusBadRequest,
			expectedVideos: 0,
		},
		{
			name:           "Invalid limit",
			query:          "user_id=user1&limit=101",
			expectedStatus: http.StatusBadRequest,
			expectedVideos: 0,
		},
		{
			name:           "No follows",
			query:          "user_id=user2&limit=5",
			expectedStatus: http.StatusOK,
			expectedVideos: 3,
			expectedFirst:  "vid2", // Highest score without boost
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/api/v1/ranking/personal?"+tt.query, nil)
			rr := httptest.NewRecorder()

			err := handler.GetPersonalRanking(rr, req)
			if (err != nil) != (tt.expectedStatus != http.StatusOK) {
				t.Errorf("GetPersonalRanking error = %v, want status %d", err, tt.expectedStatus)
			}

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			if tt.expectedStatus == http.StatusOK {
				var resp httputil.HttpResponse
				if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
					t.Errorf("failed to decode response: %v", err)
				}
				videos, ok := resp.Data.([]interface{})
				if !ok || len(videos) != tt.expectedVideos {
					t.Errorf("expected %d videos, got %d", tt.expectedVideos, len(videos))
				}
				if tt.expectedVideos > 0 {
					firstVideo := videos[0].(map[string]interface{})
					if firstVideo["id"] != tt.expectedFirst {
						t.Errorf("expected first video %s, got %s", tt.expectedFirst, firstVideo["id"])
					}
				}
			})
		})
	}
}
