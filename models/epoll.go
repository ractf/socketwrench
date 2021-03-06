package models

import (
	"net"
	"reflect"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"
)

type Epoll struct {
	fd          int
	Connections map[int]*net.Conn
	Lock        *sync.RWMutex
}

func MkEpoll() (*Epoll, error) {
	fd, err := unix.EpollCreate1(0)
	if err != nil {
		return nil, err
	}
	return &Epoll{
		fd:          fd,
		Lock:        &sync.RWMutex{},
		Connections: make(map[int]*net.Conn),
	}, nil
}

func (me *Epoll) Add(conn *net.Conn) error {
	// Extract file descriptor associated with the connection
	fd := websocketFD(conn)
	err := unix.EpollCtl(me.fd, syscall.EPOLL_CTL_ADD, fd, &unix.EpollEvent{Events: unix.POLLIN | unix.POLLHUP, Fd: int32(fd)})
	if err != nil {
		return err
	}
	me.Lock.Lock()
	me.Connections[fd] = conn
	me.Lock.Unlock()
	return nil
}

func (me *Epoll) Remove(conn *net.Conn) error {
	fd := websocketFD(conn)
	err := unix.EpollCtl(me.fd, syscall.EPOLL_CTL_DEL, fd, nil)
	if err != nil {
		return err
	}
	me.Lock.Lock()
	delete(me.Connections, fd)
	me.Lock.Unlock()
	return nil
}

func (me *Epoll) Wait() ([]*net.Conn, error) {
	events := make([]unix.EpollEvent, 100)
	n, err := unix.EpollWait(me.fd, events, 100)
	if err != nil {
		return nil, err
	}
	me.Lock.RLock()
	defer me.Lock.RUnlock()
	var connections []*net.Conn
	for i := 0; i < n; i++ {
		conn := me.Connections[int(events[i].Fd)]
		connections = append(connections, conn)
	}
	return connections, nil
}

func websocketFD(conn *net.Conn) int {
	tcpConn := reflect.Indirect(reflect.ValueOf(*conn)).FieldByName("conn")
	fdVal := tcpConn.FieldByName("fd")
	pfdVal := reflect.Indirect(fdVal).FieldByName("pfd")

	return int(pfdVal.FieldByName("Sysfd").Int())
}
