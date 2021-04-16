package socket

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/ractf/socketwrench/external"
	"github.com/ractf/socketwrench/models"
	"golang.org/x/net/websocket"
)

// RunSocket runs as a goroutine for the length of the socket connection
func RunSocket(ws *websocket.Conn, id uint64, unl *sync.Mutex, msgs <-chan string, authme chan<- models.AuthMe) (uint64, bool) {
	unl.Unlock()
	log.Println("New ws connection, id", id)
	recv := make(chan string)
	die := make(chan struct{})
	var user uint32
	userset := false
	go func() { // This is safe, https://pkg.go.dev/golang.org/x/net/websocket#Conn
		defer close(die)
		for {
			var msg string
			err := websocket.Message.Receive(ws, &msg)
			if err != nil {
				return
			}
			recv <- msg
		}
	}()

	for {
		select {
		case msg := <-recv:
			log.Printf("Received on %d: %s\n", id, msg)
			var auth models.Auth
			err := json.Unmarshal([]byte(msg), &auth)
			if err == nil { // Valid packet for auth was provided
				user, userset = external.GetUser(auth.Token)
				if userset {
					log.Printf("Binding socket %d to userid %d\n", id, user)
					authme <- models.AuthMe{Id: id, Userid: user}
				}
			} else {
				log.Println(err)
			}
		case sendme := <-msgs:
			if len(sendme) > 0 {
				log.Printf("Sending on %d: %s\n", id, sendme)
				websocket.Message.Send(ws, sendme)
			}
		case <-die:
			ws.Close()
			return id, userset
		}
	}
}
