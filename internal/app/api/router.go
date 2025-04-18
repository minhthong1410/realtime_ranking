package api

import (
	httpSwagger "github.com/swaggo/http-swagger"
	"realtime_ranking/internal/handler"
)

func (api *ApiApplication) setUpRoute() {
	handler.NewRankingHandler(api.mux, api.rdb, api.logger)
	api.mux.Handle("/swagger/", httpSwagger.WrapHandler)
}
