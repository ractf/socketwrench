package external

import (
	"context"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/ractf/socketwrench/config"
)

// Redis makes a connection to Redis
func Redis() <-chan *redis.Message {
	redisClient := redis.NewClient(&redis.Options{
		Addr: config.RedisAddr,
		DB:   0,
	})

	err := redisClient.Ping(context.Background()).Err()
	if err != nil {
		log.Println("Reconnecting to redis...")
		time.Sleep(3 * time.Second)
		err := redisClient.Ping(context.Background()).Err()
		if err != nil {
			log.Fatal(err)
		}
	}
	log.Println("Connected to redis")

	ctx := context.Background()
	topic := redisClient.Subscribe(ctx, config.RedisSub)
	return topic.Channel()
}
