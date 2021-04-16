package main

import (
	"encoding/binary"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/ractf/socketwrench/config"
	"github.com/ractf/socketwrench/external"
	"github.com/ractf/socketwrench/models"
	"github.com/ractf/socketwrench/socket"

	"golang.org/x/net/websocket"
)

func listener() { // Serve HTTP
	err := http.ListenAndServe(config.WebsocketAddr, nil)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	log.Println("--- Main starting ---")
	var connections models.SafeConnMap
	var users models.SafeUserMap
	var cid uint64 = 0
	active := 0
	connections.V = make(map[uint64]chan string)
	users.V = make(map[uint32][]chan string)
	authme := make(chan models.AuthMe)
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	fromredis := external.Redis()

	http.Handle(config.HttpPath, websocket.Handler(func(ws *websocket.Conn) { // Setup handler that spawns WS goroutines
		msgs := make(chan string, 2) // Bigger capacity -> higher memory but even smaller chance of errors
		connections.Mu.Lock()
		for {
			_, present := connections.V[cid]
			if !present {
				break
			}
			cid++
		}
		connections.V[cid] = msgs
		active++
		id, userset := socket.RunSocket(ws, cid, &connections.Mu, msgs, authme)

		log.Println("Closing ws", id)
		connections.Mu.Lock() // Order matters: remove references to the channel, then clear, then close
		active--
		delete(connections.V, id)
		connections.Mu.Unlock()

		for len(msgs) > 0 {
			<-msgs
		}
		close(msgs)

		if userset {
			users.ClearUser(msgs)
		}
	}))

	go listener()
	log.Println("--- Ready to accept connections ---")

	for { // Main event listener and coordinator
		select {
		case msg := <-fromredis:
			packet := msg.Payload
			log.Println("Redis message:", packet, ":", hex.EncodeToString([]byte(packet)))
			switch int(packet[0]) {
			case 0: // Global message
				connections.SendAll(packet[1:])

			case 1: // user message
				usersend := binary.BigEndian.Uint32([]byte(packet[1:5]))
				users.SendTo(usersend, packet[5:])
			}

		case am := <-authme: // WS gave a valid token, can be authed as a user
			connections.Mu.Lock()
			mych := connections.V[am.Id]
			connections.Mu.Unlock()
			users.Update(am.Userid, mych)

		case <-interrupt:
			log.Println("--- OS INTERRUPT: Exiting gracefully in 3s. ---")
			connections.SendAll("{\"close\": 0}")
			connections.Mu.Lock() // No new connections
			time.Sleep(3 * time.Second)
			return
		}
	}
}
