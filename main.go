package main

import (
	"log"
	"runtime"
	"time"

	"github.com/ractf/socketwrench/server"
)

func statusUpdates(cues <-chan time.Time) {
	for range cues {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		server.Epoller.Lock.RLock()
		log.Printf("Goroutines = %d\tAlloc = %dKiB\tGCs = %d\tConnections = %d", runtime.NumGoroutine(), m.Alloc/1024, m.NumGC, len(server.Epoller.Connections))
		server.Epoller.Lock.RUnlock()
	}
}

func main() {
	ticker := time.NewTicker(time.Second * 5)
	go statusUpdates(ticker.C)

	server.Run() // blocks
}
