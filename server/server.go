package server

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"syscall"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/ractf/socketwrench/config"
	"github.com/ractf/socketwrench/external"
	"github.com/ractf/socketwrench/models"
)

var Epoller *epoll
var authed models.SafeAuthMap
var AuthedRev models.SafeAuthRevMap

func wsHandler(w http.ResponseWriter, r *http.Request) {
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
	log.Printf("Starting up")
	// Increase resources limitations
	var rLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		panic(err)
	}
	rLimit.Cur = rLimit.Max
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		panic(err)
	}

	// Start epoll
	var err error
	Epoller, err = MkEpoll()
	if err != nil {
		panic(err)
	}

	authed.V = make(map[uint32][]*net.Conn)
	AuthedRev.V = make(map[*net.Conn]uint32)

	go recvHandler()
	go sendHandler()

	http.HandleFunc("/", wsHandler)
	if err := http.ListenAndServe(config.WebsocketAddr, nil); err != nil {
		log.Fatal(err)
	}
}

func kill(conn *net.Conn) {
	if err := Epoller.Remove(conn); err != nil {
		log.Printf("Failed to remove %v", err)
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
	fromredis := external.Redis()
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
		}
		for _, grave := range graveyard {
			kill(grave)
		}
	}
}

func recvHandler() { // must run synchronously
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
			msg, _, err := wsutil.ReadClientData(*conn)
			if err != nil {
				kill(conn)
				continue
			}
			// Valid message from a client here
			var auth models.Auth
			err = json.Unmarshal(msg, &auth)
			if err == nil { // Valid packet for auth was provided
				user, ok := external.GetUser(auth.Token)
				if ok {
					authed.Mu.Lock()
					AuthedRev.Mu.Lock()
					log.Printf("Socket %p has authenticated as %d", conn, user)
					if _, found := AuthedRev.V[conn]; !found { // make sure connection already authed
						if _, found := authed.V[user]; found { // if userid already has connections
							authed.V[user] = append(authed.V[user], conn)
						} else {
							authed.V[user] = make([]*net.Conn, 1)
							authed.V[user][0] = conn
						}
						AuthedRev.V[conn] = user
					}
					AuthedRev.Mu.Unlock()
					authed.Mu.Unlock()
				}
			} else {
				log.Println(err)
			}
		}
	}
}
