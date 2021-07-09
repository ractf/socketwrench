package models

import (
	"net"
	"sync"
)

type SafeAuthMap struct {
	V  map[uint32][]*net.Conn
	Mu sync.Mutex
}

type SafeAuthRevMap struct {
	V  map[*net.Conn]uint32
	Mu sync.Mutex
}

type SafeReqMap struct {
	V  map[string]*net.Conn
	Mu sync.Mutex
}

type Auth struct {
	Token string `json:"token"`
}
