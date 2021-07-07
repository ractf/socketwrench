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
		server.AuthedRev.Mu.Lock()
		log.Printf("Goroutines = %d   Alloc = %dKiB   GCs = %d   Conns = %d   Authed = %d",
			runtime.NumGoroutine(), m.Alloc/1024, m.NumGC, len(server.Epoller.Connections), len(server.AuthedRev.V))
		server.AuthedRev.Mu.Unlock()
		server.Epoller.Lock.RUnlock()
	}
}

func main() {
	ticker := time.NewTicker(time.Second * 4)
	go statusUpdates(ticker.C)

	server.Run() // blocks
}
