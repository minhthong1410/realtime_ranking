package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"realtime_ranking/pkg/httputil"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func setupTest(t *testing.T) (*RankingHandler, *miniredis.Miniredis, *zap.Logger) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	handler := &RankingHandler{
		redis:  client,
		logger: logger,
	}

	return handler, mr, logger
}

func TestGetRanking(t *testing.T) {
	handler, mr, _ := setupTest(t)
	defer mr.Close()

	videoID1 := "video1"
	videoID2 := "video2"
	mr.HSet(fmt.Sprintf("video:%s", videoID1), "title", "Video One", "creator_id", "creator1", "score", "100")
	mr.HSet(fmt.Sprintf("video:%s", videoID2), "title", "Video Two", "creator_id", "creator2", "score", "50")
	mr.ZAdd("rankings:global", 100, videoID1)
	mr.ZAdd("rankings:global", 50, videoID2)

	t.Run("success", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/api/v1/ranking?limit=2", nil)
		require.NoError(t, err)
		rr := httptest.NewRecorder()

		err = handler.GetRanking(rr, req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, rr.Code)

		var response httputil.HttpResponse
		err = json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err)

		videos, ok := response.Data.([]interface{})
		assert.True(t, ok)
		assert.Len(t, videos, 2)

		video1, ok := videos[0].(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, videoID1, video1["id"])
		assert.Equal(t, "Video One", video1["title"])
		assert.Equal(t, "creator1", video1["creator_id"])
		assert.Equal(t, 100.0, video1["score"])
	})

	t.Run("invalid limit", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/api/v1/ranking?limit=101", nil)
		require.NoError(t, err)
		rr := httptest.NewRecorder()

		err = handler.GetRanking(rr, req)
		assert.ErrorIs(t, err, ErrorLimitRange)

		var response httputil.ErrorResponse
		err = json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("invalid offset", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/api/v1/ranking?offset=-1", nil)
		require.NoError(t, err)
		rr := httptest.NewRecorder()

		err = handler.GetRanking(rr, req)
		assert.ErrorIs(t, err, ErrorOffsetRange)

		var response httputil.ErrorResponse
		err = json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})
}

func TestUpdateScore(t *testing.T) {
	handler, mr, _ := setupTest(t)
	defer mr.Close()

	videoID := "video1"
	creatorID := "creator1"
	mr.HSet(fmt.Sprintf("video:%s", videoID), "title", "Video One", "creator_id", creatorID, "score", "0")

	t.Run("success like", func(t *testing.T) {
		interaction := Interaction{
			VideoID:   videoID,
			Type:      InteractionLike,
			UserID:    "user1",
			Timestamp: 1690000000,
		}
		body, _ := json.Marshal(interaction)
		req, err := http.NewRequest("POST", "/api/v1/interaction", bytes.NewReader(body))
		require.NoError(t, err)
		rr := httptest.NewRecorder()

		err = handler.UpdateScore(rr, req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, rr.Code)

		var response httputil.HttpResponse
		err = json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err)

		data, ok := response.Data.(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, 5.0, data["new_score"])

		score, err := mr.ZScore("rankings:global", videoID)
		require.NoError(t, err)
		assert.Equal(t, 5.0, score)

		creatorScore, err := mr.ZScore(fmt.Sprintf("creator:%s:videos", creatorID), videoID)
		require.NoError(t, err)
		assert.Equal(t, 5.0, creatorScore)

		videoScore := mr.HGet(fmt.Sprintf("video:%s", videoID), "score")
		assert.Equal(t, "5", videoScore)
	})

	t.Run("success watch with watch time", func(t *testing.T) {
		interaction := Interaction{
			VideoID:   videoID,
			Type:      InteractionWatch,
			UserID:    "user1",
			Timestamp: 1690000000,
			WatchTime: 120,
		}
		body, _ := json.Marshal(interaction)
		req, err := http.NewRequest("POST", "/api/v1/interaction", bytes.NewReader(body))
		require.NoError(t, err)
		rr := httptest.NewRecorder()

		err = handler.UpdateScore(rr, req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, rr.Code)

		var response httputil.HttpResponse
		err = json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err)

		data, ok := response.Data.(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, 9.0, data["new_score"])
	})

	t.Run("invalid interaction type", func(t *testing.T) {
		interaction := Interaction{
			VideoID:   videoID,
			Type:      "invalid",
			UserID:    "user1",
			Timestamp: 1690000000,
		}
		body, _ := json.Marshal(interaction)
		req, err := http.NewRequest("POST", "/api/v1/interaction", bytes.NewReader(body))
		require.NoError(t, err)
		rr := httptest.NewRecorder()

		err = handler.UpdateScore(rr, req)
		assert.ErrorIs(t, err, ErrorInvalidInteractionType)

		var response httputil.ErrorResponse
		err = json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("invalid video id", func(t *testing.T) {
		interaction := Interaction{
			VideoID:   "",
			Type:      InteractionLike,
			UserID:    "user1",
			Timestamp: 1690000000,
		}
		body, _ := json.Marshal(interaction)
		req, err := http.NewRequest("POST", "/api/v1/interaction", bytes.NewReader(body))
		require.NoError(t, err)
		rr := httptest.NewRecorder()

		err = handler.UpdateScore(rr, req)
		assert.ErrorIs(t, err, ErrorInvalidVideoID)

		var response httputil.ErrorResponse
		err = json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("invalid timestamp", func(t *testing.T) {
		interaction := Interaction{
			VideoID:   videoID,
			Type:      InteractionLike,
			UserID:    "user1",
			Timestamp: 0,
		}
		body, _ := json.Marshal(interaction)
		req, err := http.NewRequest("POST", "/api/v1/interaction", bytes.NewReader(body))
		require.NoError(t, err)
		rr := httptest.NewRecorder()

		err = handler.UpdateScore(rr, req)
		assert.ErrorIs(t, err, ErrorInvalidTimestamp)

		var response httputil.ErrorResponse
		err = json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})
}

func TestGetPersonalRanking(t *testing.T) {
	handler, mr, _ := setupTest(t)
	defer mr.Close()

	userID := "user1"
	creator1 := "creator1"
	creator2 := "creator2"
	video1 := "video1"
	video2 := "video2"
	video3 := "video3"

	mr.SAdd(fmt.Sprintf("user:%s:follows", userID), creator1)
	mr.SAdd(fmt.Sprintf("user:%s:interactions", userID), video2)

	mr.HSet(fmt.Sprintf("video:%s", video1), "title", "Video One", "creator_id", creator1, "score", "100")
	mr.HSet(fmt.Sprintf("video:%s", video2), "title", "Video Two", "creator_id", creator2, "score", "80")
	mr.HSet(fmt.Sprintf("video:%s", video3), "title", "Video Three", "creator_id", creator2, "score", "60")

	mr.ZAdd("rankings:global", 100, video1)
	mr.ZAdd("rankings:global", 80, video2)
	mr.ZAdd("rankings:global", 60, video3)

	mr.ZAdd(fmt.Sprintf("creator:%s:videos", creator1), 100, video1)
	mr.ZAdd(fmt.Sprintf("creator:%s:videos", creator2), 80, video2)
	mr.ZAdd(fmt.Sprintf("creator:%s:videos", creator2), 60, video3)

	t.Run("success", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/api/v1/ranking/personal?user_id=user1&limit=2", nil)
		require.NoError(t, err)
		rr := httptest.NewRecorder()

		err = handler.GetPersonalRanking(rr, req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, rr.Code)

		var response httputil.HttpResponse
		err = json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err)

		videos, ok := response.Data.([]interface{})
		assert.True(t, ok)
		assert.Len(t, videos, 2)

		video1Data, ok := videos[0].(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, video1, video1Data["id"])
		assert.Equal(t, 100.0, video1Data["score"])

		video2Data, ok := videos[1].(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, video2, video2Data["id"])
		assert.Equal(t, 80.0, video2Data["score"])
	})

	t.Run("missing user_id", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/api/v1/ranking/personal", nil)
		require.NoError(t, err)
		rr := httptest.NewRecorder()

		err = handler.GetPersonalRanking(rr, req)
		assert.ErrorIs(t, err, ErrorUserIDMissing)

		var response httputil.ErrorResponse
		err = json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("invalid limit", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/api/v1/ranking/personal?user_id=user1&limit=101", nil)
		require.NoError(t, err)
		rr := httptest.NewRecorder()

		err = handler.GetPersonalRanking(rr, req)
		assert.ErrorIs(t, err, ErrorLimitRange)

		var response httputil.ErrorResponse
		err = json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})
}
