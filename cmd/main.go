package cmd

import (
	"context"
	"os/signal"
	"realtime_ranking/internal/app/api"
	"realtime_ranking/pkg/logutil"
	"realtime_ranking/pkg/redis"
	"syscall"
)

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
