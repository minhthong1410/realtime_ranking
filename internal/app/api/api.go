package api

import (
	"context"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"log"
	"net/http"

	"go.uber.org/zap"

	"realtime_ranking/pkg/middleware"
)

type Application interface {
	Start()
	Shutdown()
}

type ApiApplication struct {
	ctx    context.Context
	logger *zap.Logger
	srv    *http.Server
	mux    *http.ServeMux
	rdb    *redis.Client
}

func (api *ApiApplication) Start() {
	go func() {
		api.logger.Sugar().Infof("start server %v", api.srv.Addr)
		if err := api.srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen and serve returned err: %v", err)
		}
		api.logger.Sugar().Infof("stopped serving new connections.")
	}()
}

func (api *ApiApplication) Shutdown() {
	// shutdownCtx, shutdownRelease := context.WithTimeout(context.Background(), 2*time.Second)
	// defer shutdownRelease()
	api.logger.Info("shutting down API")
	if err := api.srv.Shutdown(api.ctx); err != nil {
		api.logger.Error("shutdown api", zap.Error(err))
	}

	api.logger.Info("Shutting down... Closing Redis connection.")
	if err := api.rdb.Close(); err != nil {
		api.logger.Error("Redis closed error", zap.Error(err))
	} else {
		api.logger.Info("Redis closed successfully.")
	}
	// <-shutdownCtx.Done()

	api.logger.Info("shut down")
}

func NewRouter(logger *zap.Logger, errorHandler middleware.OnError) (*http.ServeMux, *http.Server) {
	mux := &http.ServeMux{}
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", 8080),
		Handler: middleware.LoggerWrap(middleware.RecoverWrap(mux, errorHandler), logger),
	}
	return mux, srv
}

func NewApiApplication(ctx context.Context, logger *zap.Logger, client *redis.Client) *ApiApplication {
	application := &ApiApplication{ctx: ctx, logger: logger, rdb: client}
	mux, srv := NewRouter(logger, application.errorHandler)
	application.mux = mux
	application.srv = srv

	application.setUpRoute()
	return application
}
