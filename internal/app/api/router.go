package api

import "realtime_ranking/internal/handler"

func (api *ApiApplication) setUpRoute() {
	handler.NewRankingHandler(api.mux, api.rdb, api.logger)
}
