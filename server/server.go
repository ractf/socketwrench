package server

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/ractf/socketwrench/external"
	"github.com/ractf/socketwrench/models"
)

var Epoller *models.Epoll
var toredis chan []byte
var authed models.SafeAuthMap
var AuthedRev models.SafeAuthRevMap
var AuthReqs models.SafeReqMap

func WsHandler(w http.ResponseWriter, r *http.Request) {
	// Upgrade connection
	conn, _, _, err := ws.UpgradeHTTP(r, w)
	if err != nil {
		return
	}
	if err := Epoller.Add(&conn); err != nil {
		log.Printf("Failed to add connection %v", err)
		conn.Close()
	}
}

func Run() {
	// Start epoll
	var err error
	Epoller, err = models.MkEpoll()
	if err != nil {
		panic(err)
	}

	external.Setup()
	go external.SendOn(toredis)

	authed.V = make(map[uint32][]*net.Conn)
	AuthedRev.V = make(map[*net.Conn]uint32)
	AuthReqs.V = make(map[string]*net.Conn)

	go recvHandler()
	go sendHandler()
	log.Println("socketwrench ready to go.")
}

func kill(conn *net.Conn) {
	if err := Epoller.Remove(conn); err != nil {
		if fmt.Sprint(err) != "bad file descriptor" { // shit happens
			log.Printf("Failed to remove %v", err)
		}
	}
	(*conn).Close()
	AuthedRev.Mu.Lock()
	if uid, ok := AuthedRev.V[conn]; ok {
		delete(AuthedRev.V, conn)

		authed.Mu.Lock()
		if conns, found := authed.V[uid]; found {
			delind := -1
			for ind, val := range conns {
				if val == conn {
					delind = ind
					break
				}
			}
			if delind > -1 {
				conns[delind] = conns[len(conns)-1]
				conns = conns[:len(conns)-1]
			}
			if len(conns) == 0 {
				delete(authed.V, uid)
			}
		}
		authed.Mu.Unlock()
	}
	AuthedRev.Mu.Unlock()
}

func sendHandler() {
	fromredis := external.GetRecv()
	for msg := range fromredis {
		packet := msg.Payload
		log.Println("Redis message:", hex.EncodeToString([]byte(packet)))
		graveyard := make([]*net.Conn, 0)
		switch packet[0] {
		case 0: // Global message
			send := []byte(packet[1:])
			Epoller.Lock.RLock()
			for _, conn := range Epoller.Connections {
				err := wsutil.WriteServerText(*conn, send)
				if err != nil {
					graveyard = append(graveyard, conn)
				}
			}
			Epoller.Lock.RUnlock()

		case 1: // user message
			usersend := binary.BigEndian.Uint32([]byte(packet[1:5]))
			authed.Mu.Lock()
			if conns, ok := authed.V[usersend]; ok {
				for _, conn := range conns {
					err := wsutil.WriteServerText(*conn, []byte(packet[5:]))
					if err != nil {
						graveyard = append(graveyard, conn)
					}
				}
			}
			authed.Mu.Unlock()

		case 2: // auth failed
			AuthReqs.Mu.Lock()
			delete(AuthReqs.V, packet[1:])
			AuthReqs.Mu.Unlock()

		case 3: // auth success
			uid := binary.BigEndian.Uint32([]byte(packet[1:5]))
			AuthReqs.Mu.Lock()
			conn, ok := AuthReqs.V[packet[5:]]
			delete(AuthReqs.V, packet[5:])
			AuthReqs.Mu.Unlock()
			if ok {
				authed.Mu.Lock()
				AuthedRev.Mu.Lock()
				//log.Printf("Socket %p has authenticated as %d", conn, user)
				if _, found := AuthedRev.V[conn]; !found { // make sure connection already authed
					if _, found := authed.V[uid]; found { // if userid already has connections
						authed.V[uid] = append(authed.V[uid], conn)
					} else {
						authed.V[uid] = make([]*net.Conn, 1)
						authed.V[uid][0] = conn
					}
					AuthedRev.V[conn] = uid
				}
				AuthedRev.Mu.Unlock()
				authed.Mu.Unlock()
			}
		}
		for _, grave := range graveyard {
			kill(grave)
		}
	}
}

func recvHandler() {
	for {
		connections, err := Epoller.Wait()
		if err != nil {
			if fmt.Sprint(err) != "interrupted system call" { // shit happens
				log.Println(err)
			}
			continue
		}
		for _, conn := range connections {
			if conn == nil {
				break
			}
			if *conn == nil {
				break
			}
			msg, _, err := wsutil.ReadClientData(*conn)
			if err != nil {
				kill(conn)
				continue
			}
			// Valid message from a client here
			var auth models.Auth
			err = json.Unmarshal(msg, &auth)
			if err == nil { // Valid packet for auth was provided
				packet := make([]byte, 0, 40)
				packet = append(packet, 128)
				packet = append(packet, auth.Token...)
				toredis <- packet
				AuthReqs.Mu.Lock()
				AuthReqs.V[auth.Token] = conn
				AuthReqs.Mu.Unlock()

			} else {
				log.Println(err)
			}
		}
	}
}
