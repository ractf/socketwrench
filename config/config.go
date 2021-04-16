package config

import "os"

var RedisAddr, RedisSub, HttpPath, WebsocketAddr, BackendAddr string

func init() {
	var ok bool
	RedisAddr, ok = os.LookupEnv("REDIS_ADDR")
	if !ok {
		panic("Missing env var REDIS_ADDR")
	}
	RedisSub, ok = os.LookupEnv("REDIS_SUB")
	if !ok {
		panic("Missing env var REDIS_SUB")
	}
	HttpPath, ok = os.LookupEnv("HTTP_PATH")
	if !ok {
		panic("Missing env var HTTP_PATH")
	}
	WebsocketAddr, ok = os.LookupEnv("WEBSOCKET_ADDR")
	if !ok {
		panic("Missing env var WEBSOCKET_ADDR")
	}
	BackendAddr, ok = os.LookupEnv("BACKEND_ADDR")
	if !ok {
		panic("Missing env var BACKEND_ADDR")
	}
}
