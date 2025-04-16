package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"realtime_ranking/pkg/httputil"
	"realtime_ranking/pkg/middleware"
	"sort"
	"strconv"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Constants for interaction types and their score increments
const (
	InteractionView    = "view"
	InteractionLike    = "like"
	InteractionComment = "comment"
	InteractionShare   = "share"
)

var scoreIncrements = map[string]float64{
	InteractionView:    1.0,
	InteractionLike:    5.0,
	InteractionComment: 10.0,
	InteractionShare:   20.0,
}

type RankingHandler struct {
	redis  *redis.Client
	logger *zap.Logger
}

type Video struct {
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	CreatorID string  `json:"creator_id"`
	Score     float64 `json:"score"`
}

type Interaction struct {
	VideoID   string `json:"video_id"`
	Type      string `json:"type"`
	UserID    string `json:"user_id"`
	Timestamp int64  `json:"timestamp"`
}

// GetRanking retrieves the global ranking of videos
func (h *RankingHandler) GetRanking(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil {
		limit = 10 // default limit
	}
	if limit > 100 || limit < 1 {
		return ErrorLimitRange
	}

	offset, err := strconv.Atoi(r.URL.Query().Get("offset"))
	if err != nil {
		offset = 0 // default offset
	}
	if offset < 0 {
		return ErrorOffsetRange
	}

	videoIDs, err := h.redis.ZRevRange(ctx, "rankings:global", int64(offset), int64(offset+limit-1)).Result()
	if err != nil {
		h.logger.Error("failed to get rankings", zap.Error(err))
		return ErrorGetDataFailed
	}

	var videos []Video
	for _, videoID := range videoIDs {
		videoData, err := h.redis.HGetAll(ctx, fmt.Sprintf("video:%s", videoID)).Result()
		if err != nil {
			h.logger.Info("failed to get video data", zap.String("video_id", videoID), zap.Error(err))
			return ErrorGetDataFailed
		}
		score, _ := strconv.ParseFloat(videoData["score"], 64)
		video := Video{
			ID:        videoID,
			Title:     videoData["title"],
			CreatorID: videoData["creator_id"],
			Score:     score,
		}
		videos = append(videos, video)
	}
	return httputil.RenderJSON(http.StatusOK, w, httputil.HttpResponse{
		Code: http.StatusOK,
		Data: videos,
	})
}

// UpdateScore updates a video's score based on user interaction
func (h *RankingHandler) UpdateScore(w http.ResponseWriter, r *http.Request) error {
	var interaction Interaction
	if err := json.NewDecoder(r.Body).Decode(&interaction); err != nil {
		return ErrorInvalidRequestBody
	}

	increment, ok := scoreIncrements[interaction.Type]
	if !ok {
		return ErrorInvalidInteractionType
	}

	ctx := r.Context()
	videoKey := fmt.Sprintf("video:%s", interaction.VideoID)
	creatorID, err := h.redis.HGet(ctx, videoKey, "creator_id").Result()
	if err != nil {
		h.logger.Info("failed to get video data", zap.Error(err))
		return ErrorGetDataFailed
	}

	// Update global ranking
	globalKey := "rankings:global"
	newScore, err := h.redis.ZIncrBy(ctx, globalKey, increment, interaction.VideoID).Result()
	if err != nil {
		h.logger.Info("failed to update global ranking", zap.Error(err))
		return ErrorUpdateDataFailed
	}

	// Update creator-specific ranking
	creatorKey := fmt.Sprintf("creator:%s:videos", creatorID)
	_, err = h.redis.ZIncrBy(ctx, creatorKey, increment, interaction.VideoID).Result()
	if err != nil {
		h.logger.Info("failed to update creator ranking", zap.Error(err))
		return ErrorUpdateDataFailed
	}

	// Update score in video hash for consistency
	err = h.redis.HSet(ctx, videoKey, "score", newScore).Err()
	if err != nil {
		h.logger.Info("failed to update score in video hash", zap.Error(err))
		return ErrorUpdateDataFailed
	}

	return httputil.RenderJSON(http.StatusOK, w, httputil.HttpResponse{
		Code: http.StatusOK,
		Data: map[string]interface{}{"new_score": newScore},
	})
}

// GetPersonalRanking retrieves a personalized ranking for a user
func (h *RankingHandler) GetPersonalRanking(w http.ResponseWriter, r *http.Request) error {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		return ErrorUserIDMissing
	}

	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil {
		limit = 20 // default limit
	}
	if limit > 100 || limit < 1 {
		return ErrorLimitRange
	}

	ctx := r.Context()
	followKey := fmt.Sprintf("user:%s:follows", userID)
	followedCreators, err := h.redis.SMembers(ctx, followKey).Result()
	if err != nil {
		h.logger.Info("failed to get followed creators", zap.Error(err))
		return ErrorGetDataFailed
	}

	// Collect candidate videos
	const topKPerCreator = 10
	const topMGlobal = 50
	var candidateVideos []string
	for _, creatorID := range followedCreators {
		creatorKey := fmt.Sprintf("creator:%s:videos", creatorID)
		videos, err := h.redis.ZRevRange(ctx, creatorKey, 0, topKPerCreator-1).Result()
		if err != nil {
			h.logger.Info("failed to get videos for creator", zap.String("creator_id", creatorID), zap.Error(err))
			return ErrorGetDataFailed
		}
		candidateVideos = append(candidateVideos, videos...)
	}

	// Add top global videos
	globalVideos, err := h.redis.ZRevRange(ctx, "rankings:global", 0, topMGlobal-1).Result()
	if err != nil {
		h.logger.Info("failed to get global rankings", zap.Error(err))
		return ErrorGetDataFailed
	}
	candidateVideos = append(candidateVideos, globalVideos...)

	// Remove duplicates
	videoSet := make(map[string]struct{})
	for _, videoID := range candidateVideos {
		videoSet[videoID] = struct{}{}
	}
	var videoIDs []string
	for videoID := range videoSet {
		videoIDs = append(videoIDs, videoID)
	}

	// Fetch scores
	scores, err := h.redis.ZMScore(ctx, "rankings:global", videoIDs...).Result()
	if err != nil {
		h.logger.Info("failed to get scores", zap.Error(err))
		return ErrorGetDataFailed
	}
	scoreMap := make(map[string]float64)
	for i, videoID := range videoIDs {
		scoreMap[videoID] = scores[i]
	}

	// Fetch creator IDs using pipeline
	pipe := h.redis.Pipeline()
	creatorCmds := make(map[string]*redis.StringCmd)
	for videoID := range videoSet {
		videoKey := fmt.Sprintf("video:%s", videoID)
		creatorCmds[videoID] = pipe.HGet(ctx, videoKey, "creator_id")
	}
	_, err = pipe.Exec(ctx)
	if err != nil {
		h.logger.Info("failed to get creator IDs", zap.Error(err))
		return ErrorGetDataFailed
	}

	creatorMap := make(map[string]string)
	for videoID, cmd := range creatorCmds {
		if creatorID, err := cmd.Result(); err == nil {
			creatorMap[videoID] = creatorID
		}
	}

	// Apply boosts and sort
	const boost = 100.0

	var adjustedScores []VideoScore
	for videoID := range videoSet {
		score := scoreMap[videoID]
		if creatorID, ok := creatorMap[videoID]; ok && contains(followedCreators, creatorID) {
			score += boost
		}
		adjustedScores = append(adjustedScores, VideoScore{videoID, score})
	}
	sort.Slice(adjustedScores, func(i, j int) bool {
		return adjustedScores[i].Score > adjustedScores[j].Score
	})

	// Limit to top N
	if len(adjustedScores) > limit {
		adjustedScores = adjustedScores[:limit]
	}

	// Fetch video details
	var videos []Video
	for _, item := range adjustedScores {
		videoKey := fmt.Sprintf("video:%s", item.VideoID)
		videoData, err := h.redis.HGetAll(ctx, videoKey).Result()
		if err != nil {
			h.logger.Info("failed to get video data", zap.String("video_id", item.VideoID), zap.Error(err))
			return ErrorGetDataFailed
		}
		score, _ := strconv.ParseFloat(videoData["score"], 64)
		videos = append(videos, Video{
			ID:        item.VideoID,
			Title:     videoData["title"],
			CreatorID: videoData["creator_id"],
			Score:     score, // Return original score
		})
	}

	return httputil.RenderJSON(http.StatusOK, w, httputil.HttpResponse{
		Code: http.StatusOK,
		Data: videos,
	})
}

type VideoScore struct {
	VideoID string
	Score   float64
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// NewRankingHandler sets up all routes
func NewRankingHandler(mux *http.ServeMux, redis *redis.Client, logger *zap.Logger) {
	handler := &RankingHandler{
		redis:  redis,
		logger: logger,
	}
	mux.HandleFunc("GET /api/v1/ranking", middleware.WithErrorHandler(handler.GetRanking, logger))
	mux.HandleFunc("POST /api/v1/interaction", middleware.WithErrorHandler(handler.UpdateScore, logger))
	mux.HandleFunc("GET /api/v1/ranking/personal", middleware.WithErrorHandler(handler.GetPersonalRanking, logger))
}
