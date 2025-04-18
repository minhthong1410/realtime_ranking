package main

import (
	"context"
	"os/signal"
	_ "realtime_ranking/docs"
	"realtime_ranking/internal/app/api"
	"realtime_ranking/pkg/logutil"
	"realtime_ranking/pkg/redis"
	"syscall"
)

// @title			Realtime Ranking API
// @version		1.0
// @description	Realtime ranking API
// @host			localhost:8080
// @BasePath		/api/v1
func main() {
	logger := logutil.InitLogger()
	defer logger.Sync()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	redisClient := redis.NewRedisClient()

	application := api.NewApiApplication(ctx, logger, redisClient)
	application.Start()
	defer application.Shutdown()

	<-ctx.Done()
}
