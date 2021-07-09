package config

import "os"

var RedisAddr, RedisSub, WebsocketAddr, BackendAddr string

func init() {
	var ok bool
	RedisAddr, ok = os.LookupEnv("REDIS_ADDR") // where is redis, passed to redis.NewClient
	if !ok {
		panic("Missing env var REDIS_ADDR")
	}
	RedisSub, ok = os.LookupEnv("REDIS_SUB") // what is the redis pub/sub called
	if !ok {
		panic("Missing env var REDIS_SUB")
	}
	WebsocketAddr, ok = os.LookupEnv("WEBSOCKET_ADDR") // where to host the http upgrade handler, passed to http.ListenAndServe
	if !ok {
		panic("Missing env var WEBSOCKET_ADDR")
	}
	BackendAddr, ok = os.LookupEnv("BACKEND_ADDR") // where is backend, passed to http.NewRequest, should not end in /
	if !ok {
		panic("Missing env var BACKEND_ADDR")
	}
}
