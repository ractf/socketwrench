package main

import (
	"log"
	"net/http"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/ractf/socketwrench/config"
	"github.com/ractf/socketwrench/server"
)

var concurConnections = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "socketwrench_concurrent_connections",
	Help: "The number of websockets currently connected",
})
var concurAuthed = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "socketwrench_concurrent_authed_connections",
	Help: "The number of websockets currently connected and authenticated",
})
var authedQueue = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "socketwrench_auth_queue",
	Help: "The number of authentication requests waiting on backend",
})

func stats(cues <-chan time.Time) {
	for range cues {
		server.Epoller.Lock.RLock()
		concurConnections.Set(float64(len(server.Epoller.Connections)))
		server.Epoller.Lock.RUnlock()
		server.AuthedRev.Mu.Lock()
		concurAuthed.Set(float64(len(server.AuthedRev.V)))
		server.AuthedRev.Mu.Unlock()
		server.AuthReqs.Mu.Lock()
		authedQueue.Set(float64(len(server.AuthReqs.V)))
		server.AuthReqs.Mu.Unlock()

	}

}

func main() {
	log.Println("socketwrench starting up...")
	// Increase resources limitations
	var rLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		panic(err)
	}
	rLimit.Cur = rLimit.Max
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		panic(err)
	}

	go server.Run()
	ticker := time.NewTicker(time.Second * 10)
	go stats(ticker.C)

	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/", server.WsHandler)
	if err := http.ListenAndServe(config.MyAddr, nil); err != nil {
		log.Fatal(err)
	}
}
