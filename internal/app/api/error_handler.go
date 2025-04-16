package api

import (
	"net/http"

	"realtime_ranking/pkg/httputil"

	"go.uber.org/zap"
)

func (api *ApiApplication) errorHandler(err any, w http.ResponseWriter) {
	api.logger.Error("err_unknown", zap.Any("error", err))
	if err := httputil.RenderJSON(http.StatusInternalServerError, w,
		map[string]any{
			"message": "err_unknown",
			"code":    5000,
		}); err != nil {
		api.logger.Error("render json error", zap.Error(err))
	}
}
