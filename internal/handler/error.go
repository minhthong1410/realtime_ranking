package handler

import (
	"errors"
	"net/http"
	"realtime_ranking/pkg/httputil"
)

type RankingError struct {
	Code int `json:"code"`
	Err  error
}

func (e RankingError) Error() string {
	return e.Err.Error()
}

func (e RankingError) HttpCode() int {
	return e.Code
}
func (e RankingError) HttpResponse() httputil.ErrorResponse {
	return httputil.ErrorResponse{
		Message: e.Err.Error(),
		Code:    e.Code,
	}
}

var (
	ErrorInvalidRequestBody = RankingError{
		Code: http.StatusBadRequest,
		Err:  errors.New("invalid request body"),
	}
	ErrorInvalidInteractionType = RankingError{
		Code: http.StatusBadRequest,
		Err:  errors.New("invalid interaction type"),
	}
	ErrorGetDataFailed = RankingError{
		Code: http.StatusInternalServerError,
		Err:  errors.New("failed to get data"),
	}
	ErrorUpdateDataFailed = RankingError{
		Code: http.StatusInternalServerError,
		Err:  errors.New("failed to update data"),
	}
	ErrorUserIDMissing = RankingError{
		Code: http.StatusBadRequest,
		Err:  errors.New("user_id is required"),
	}
	ErrorLimitRange = RankingError{
		Code: http.StatusBadRequest,
		Err:  errors.New("limit must be between 1 and 100"),
	}
	ErrorOffsetRange = RankingError{
		Code: http.StatusBadRequest,
		Err:  errors.New("offset must be greater than 0"),
	}
)
