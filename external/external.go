package external

import (
	"context"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/ractf/socketwrench/config"
)

var redisClient *redis.Client

func Setup() {
	redisClient = redis.NewClient(&redis.Options{
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
}

func GetRecv() <-chan *redis.Message {
	topic := redisClient.Subscribe(context.Background(), config.RedisSub)
	return topic.Channel()
}

func SendOn(msgs <-chan []byte) {
	for msg := range msgs {
		redisClient.Publish(context.Background(), config.RedisSub, msg)
	}
}
