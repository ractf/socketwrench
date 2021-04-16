package main

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"golang.org/x/net/websocket"
)

// Move these to env vars
const (
	RedisAddr     = "localhost:6379"
	RedisSub      = "websocket"
	HttpPath      = "/"
	WebsocketAddr = ":12345"
	BackendAddr   = "[put your staging env here]"
)

// SafeConnMap maps ids to channels with a mutex
type SafeConnMap struct {
	v  map[uint64]chan string
	mu sync.Mutex
}

// SafeUserMap maps userids to slices of channels with a mutex
type SafeUserMap struct {
	v  map[uint32][]chan string
	mu sync.Mutex
}

// AuthMe represents a packet sent from an authed goroutine, instructs main to reflect changes
type AuthMe struct {
	id     uint64
	userid uint32
}

// Auth parses a JSON auth request from the client
type Auth struct {
	Token string
}

// MemberResponse parses the data from backend /member/self
type MemberResponse struct {
	S bool
	D map[string]interface{}
	M string
}

func remove(s []chan string, i int) []chan string {
	s[i] = s[len(s)-1]
	return s[:len(s)-1]
}

// GetUser queries the backend for a given token
func GetUser(token string) (uint32, bool) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", BackendAddr+"member/self/", nil)
	if err != nil {
		return 0, false
	}
	req.Header.Set("Authorization", "Token "+token)
	res, err := client.Do(req)
	if err != nil {
		return 0, false
	}
	response, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return 0, false
	}
	var mr MemberResponse
	err = json.Unmarshal([]byte(response), &mr)
	if err != nil {
		return 0, false
	}
	id, pres := mr.D["id"]
	if !pres {
		return 0, false
	}
	fmt.Println(id)
	return uint32(id.(float64)), true
}

func clearuser(del chan string, users map[uint32][]chan string) {
	for userid, slc := range users {
		for index, val := range slc {
			if del == val {
				if len(slc) == 1 {
					delete(users, userid)
				} else {
					users[userid] = remove(users[userid], index)
				}
			}
		}
	}
}

// Redis makes a connection to Redis
func Redis() <-chan *redis.Message {
	redisClient := redis.NewClient(&redis.Options{
		Addr: RedisAddr,
		DB:   0,
	})

	err := redisClient.Ping(context.Background()).Err()
	if err != nil {
		log.Println("Reconnecting to redis...")
		time.Sleep(3 * time.Second)
		err := redisClient.Ping(context.Background()).Err()
		if err != nil {
			log.Fatal(err)
		}
	}
	log.Println("Connected to redis")

	ctx := context.Background()
	topic := redisClient.Subscribe(ctx, RedisSub)
	return topic.Channel()
}

// RunSocket runs as a goroutine for the length of the socket connection
func RunSocket(ws *websocket.Conn, id uint64, unl *sync.Mutex, msgs <-chan string, authme chan<- AuthMe) (uint64, bool) {
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
			var auth Auth
			err := json.Unmarshal([]byte(msg), &auth)
			if err == nil { // Valid packet for auth was provided
				user, userset = GetUser(auth.Token)
				if userset {
					log.Printf("Binding socket %d to userid %d\n", id, user)
					authme <- AuthMe{id, user}
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

func listener() { // Serve HTTP
	err := http.ListenAndServe(WebsocketAddr, nil)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	log.Println("--- Main starting ---")
	var connections SafeConnMap
	var users SafeUserMap
	var cid uint64 = 0
	active := 0
	connections.v = make(map[uint64]chan string)
	users.v = make(map[uint32][]chan string)
	authme := make(chan AuthMe)
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	fromredis := Redis()

	http.Handle(HttpPath, websocket.Handler(func(ws *websocket.Conn) { // Setup handler that spawns WS goroutines
		msgs := make(chan string) // This may need to be buffered
		connections.mu.Lock()
		for {
			_, present := connections.v[cid]
			if !present {
				break
			}
			cid++
		}
		connections.v[cid] = msgs
		active++
		id, userset := RunSocket(ws, cid, &connections.mu, msgs, authme) // Blocks for a long time

		log.Println("Closing ws", id)
		connections.mu.Lock() // Order matters: remove references to the channel, then clear, then close
		active--
		delete(connections.v, id)
		connections.mu.Unlock()

		for len(msgs) > 0 {
			<-msgs
		}
		close(msgs)

		if userset {
			users.mu.Lock()
			clearuser(msgs, users.v)
			users.mu.Unlock()
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
				connections.mu.Lock()
				for _, msgchan := range connections.v {
					log.Println(msgchan)
					msgchan <- packet[1:]
				}
				connections.mu.Unlock()

			case 1: // user message
				usersend := binary.BigEndian.Uint32([]byte(packet[1:5]))
				users.mu.Lock()
				chs, present := users.v[usersend]
				users.mu.Unlock()
				if present {
					for _, msgchan := range chs {
						log.Println(chs)
						msgchan <- packet[5:]
					}
				}
			}

		case am := <-authme: // WS gave a valid token, can be authed as a user
			users.mu.Lock()
			chs, present := users.v[am.userid]
			users.mu.Unlock()
			donothing := false
			connections.mu.Lock()
			mych := connections.v[am.id]
			connections.mu.Unlock()

			if present { // userid already in users, check if this socket is already there
				for _, ch := range chs {
					if mych == ch {
						donothing = true
						break
					}
				}
			}

			users.mu.Lock()
			if !present { // Create the slice, first time
				users.v[am.userid] = make([]chan string, 1)
				users.v[am.userid][0] = mych
			} else if !donothing {
				users.v[am.userid] = append(users.v[am.userid], mych)
			}
			users.mu.Unlock()

		case <-interrupt:
			log.Println("--- OS INTERRUPT: Exiting gracefully in 3s. ---")
			connections.mu.Lock()
			for _, msgchan := range connections.v {
				msgchan <- "{\"close\": 0}"
			}
			time.Sleep(3 * time.Second)
			return
		}
	}
}
