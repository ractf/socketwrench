package models

import (
	"sync"
)

func remove(s []chan string, i int) []chan string {
	s[i] = s[len(s)-1]
	return s[:len(s)-1]
}

// SafeConnMap maps ids to channels with a mutex
type SafeConnMap struct {
	V  map[uint64]chan string
	Mu sync.Mutex
}

// SafeUserMap maps userids to slices of channels with a mutex
type SafeUserMap struct {
	V  map[uint32][]chan string
	Mu sync.Mutex
}

// AuthMe represents a packet sent from an authed goroutine, instructs main to reflect changes
type AuthMe struct {
	Id     uint64
	Userid uint32
}

// ClearUser eliminates all references to a channel from the SafeUserMap
func (me *SafeUserMap) ClearUser(del chan string) {
	me.Mu.Lock()
	for userid, slc := range me.V {
		for index, val := range slc {
			if del == val {
				if len(slc) == 1 {
					delete(me.V, userid)
				} else {
					me.V[userid] = remove(me.V[userid], index)
				}
			}
		}
	}
	me.Mu.Unlock()
}

// SendAll sends a string down all channels in the map
func (me *SafeConnMap) SendAll(msg string) {
	me.Mu.Lock()
	for _, msgchan := range me.V {
		msgchan <- msg
	}
	me.Mu.Unlock()
}

// SendTo sends to all channels of a specific user
func (me *SafeUserMap) SendTo(userid uint32, msg string) {
	me.Mu.Lock()
	chs, present := me.V[userid]
	me.Mu.Unlock()
	if present {
		for _, msgchan := range chs {
			msgchan <- msg
		}
	}
}

// Update gracefully updates a userid with a channel
func (me *SafeUserMap) Update(userid uint32, msgs chan string) {
	me.Mu.Lock()
	chs, present := me.V[userid]

	if present { // userid already in users
		for _, ch := range chs { // check if this socket is already there
			if msgs == ch {
				me.Mu.Unlock()
				return
			}
		} // It isn't already there

		me.V[userid] = append(me.V[userid], msgs)
	} else {
		me.V[userid] = make([]chan string, 1)
		me.V[userid][0] = msgs
	}
	me.Mu.Unlock()
}
