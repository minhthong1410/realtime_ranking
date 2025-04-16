package middleware

import (
	"go.uber.org/zap"
	"net/http"
	"realtime_ranking/pkg/httputil"
)

type Endpoint func(w http.ResponseWriter, r *http.Request) error

type HttpResponseSerializable interface {
	ToHttpResponse() httputil.ErrorResponse
	ToHttpCode() int
}

func WithErrorHandler(runner Endpoint, logger *zap.Logger) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		err := runner(w, r)
		if err != nil {
			logger.Error("http error", zap.Error(err))
			switch err := err.(type) {
			case HttpResponseSerializable:
				httputil.RenderJSON(err.ToHttpCode(), w, err.ToHttpResponse())
			default:
				httputil.RenderJSON(http.StatusInternalServerError, w, httputil.ErrorResponse{
					Message: "internal server",
					Code:    http.StatusInternalServerError,
				})
			}
		}
	}
}
