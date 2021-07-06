package server

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
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

var epoller *epoll

func wsHandler(w http.ResponseWriter, r *http.Request) {
	// Upgrade connection
	conn, _, _, err := ws.UpgradeHTTP(r, w)
	if err != nil {
		return
	}
	if err := epoller.Add(conn); err != nil {
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
	epoller, err = MkEpoll()
	if err != nil {
		panic(err)
	}

	go mainHandler()

	http.HandleFunc("/", wsHandler)
	if err := http.ListenAndServe(config.WebsocketAddr, nil); err != nil {
		log.Fatal(err)
	}
}

func kill(conn net.Conn) {
	if err := epoller.Remove(conn); err != nil {
		log.Printf("Failed to remove %v", err)
	}
	conn.Close()
}

func mainHandler() { // must run synchronously
	fromredis := external.Redis()
	authed := make(map[uint32][]*net.Conn)
	for {
		connections, err := epoller.Wait()
		if err != nil {
			log.Printf("Failed to epoll wait %v", err)
			continue
		}
		for _, conn := range connections {
			if conn == nil {
				break
			}
			msg, _, err := wsutil.ReadClientData(conn)
			if err != nil {
				kill(conn)
			} else {
				// Valid message from a client here
				var auth models.Auth
				err := json.Unmarshal(msg, &auth)
				if err == nil { // Valid packet for auth was provided
					user, ok := external.GetUser(auth.Token)
					if ok {
						log.Printf("Socket %p has authenticated as %d\n", conn, user)
						if _, ok := authed[user]; ok { // if found
							authed[user] = append(authed[user], &conn)
						} else {
							authed[user] = make([]*net.Conn, 1)
							authed[user][0] = &conn
						}
					}
				} else {
					log.Println(err)
				}
			}
		}
		waiting := true
		for waiting {
			select {
			case msg := <-fromredis:
				packet := msg.Payload
				log.Println("Redis message:", hex.EncodeToString([]byte(packet)))
				switch packet[0] {
				case 0: // Global message
					send := []byte(packet[1:])
					for _, conn := range epoller.connections {
						err := wsutil.WriteServerText(conn, send)
						if err != nil {
							kill(conn)
						}
					}

				case 1: // user message
					usersend := binary.BigEndian.Uint32([]byte(packet[1:5]))
					if conns, ok := authed[usersend]; ok {
						for ind, conn := range conns {
							err := wsutil.WriteServerText(*conn, []byte(packet[5:]))
							if err != nil {
								kill(*conn)
								authed[usersend][ind] = authed[usersend][len(authed[usersend])-1]
								authed[usersend] = authed[usersend][:len(authed[usersend])-1]
								if len(authed[usersend]) == 0 {
									delete(authed, usersend)
								}
							}
						}
					}
				}

			default:
				waiting = false
			}
		}
	}
}
