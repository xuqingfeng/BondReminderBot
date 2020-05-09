package bond

import (
	"log"
	"os"

	"github.com/go-redis/redis/v7"
)

var (
	redisClient *redis.Client
)

func init() {

	redisClient = redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_ADDR"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})
	_, err := redisClient.Ping().Result()
	if err != nil {
		log.Panic(err)
	}
}
