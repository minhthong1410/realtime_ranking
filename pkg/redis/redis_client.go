package redis

import (
	"context"
	"github.com/redis/go-redis/v9"
	"log"
	"os"
	"strconv"
)

type Options struct {
	Addr     string
	Password string
	DB       int
	PoolSize int
}

func NewRedisClient() *redis.Client {

	host := os.Getenv("REDIS_ADDRESS")

	if host == "" {
		log.Fatal("failed to connect to Redis: address is nil")
		return nil
	}

	options := Options{
		Addr:     host,
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       getIntWithDefaultValue(os.Getenv("REDIS_DB"), 0),
		PoolSize: getIntWithDefaultValue(os.Getenv("REDIS_POOL"), 10),
	}

	return NewRedisClientWithOptions(&options)
}

func NewRedisClientWithOptions(options *Options) *redis.Client {

	rdb := redis.NewClient(&redis.Options{
		Addr:     options.Addr,
		Password: options.Password,
		DB:       options.DB,
		PoolSize: options.PoolSize, // Configurable connection pool size
	})

	// Check the connection
	ctx := context.Background()
	ping, err := rdb.Ping(ctx).Result()

	if err != nil {
		log.Fatal("failed to connect to Redis: error", err)
		return nil
	}

	log.Println("connect to Redis success", ping)

	return rdb
}

func getIntWithDefaultValue(str string, defaultV int) int {
	num, err := strconv.Atoi(str)
	if err != nil {
		return defaultV
	}
	return num
}
