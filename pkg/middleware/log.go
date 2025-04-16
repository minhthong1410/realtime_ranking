package middleware

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

type ResponseWriterInterceptor struct {
	http.ResponseWriter
	Status int
	Body   *bytes.Buffer
}

func LoggerWrap(h http.Handler, logger *zap.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			logger.Error("error reading request body", zap.Error(err))
			return
		}
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		o := &ResponseWriterInterceptor{ResponseWriter: w}
		h.ServeHTTP(o, r)
		elapsed := time.Since(start)
		if !strings.Contains(r.URL.Path, "/health") {
			logger.Info(
				"",
				zap.Duration("latency", elapsed),
				zap.String("method", r.Method),
				zap.String("URL", fmt.Sprint(r.URL)),
				zap.String("method", r.Proto),
				zap.Int("status", o.Status),
			)
		}
	})
}

// WriteHeader captures the status code of the response
func (w *ResponseWriterInterceptor) WriteHeader(statusCode int) {
	w.Status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// Write captures the response body
func (w *ResponseWriterInterceptor) Write(data []byte) (int, error) {
	if w.Body == nil {
		w.Body = &bytes.Buffer{}
	}
	w.Body.Write(data)
	return w.ResponseWriter.Write(data)
}
