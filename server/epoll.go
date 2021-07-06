package server

import (
	"log"
	"net"
	"reflect"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"
)

type epoll struct {
	fd          int
	connections map[int]net.Conn
	lock        *sync.RWMutex
}

func MkEpoll() (*epoll, error) {
	fd, err := unix.EpollCreate1(0)
	if err != nil {
		return nil, err
	}
	return &epoll{
		fd:          fd,
		lock:        &sync.RWMutex{},
		connections: make(map[int]net.Conn),
	}, nil
}

func (me *epoll) Add(conn net.Conn) error {
	// Extract file descriptor associated with the connection
	fd := websocketFD(conn)
	err := unix.EpollCtl(me.fd, syscall.EPOLL_CTL_ADD, fd, &unix.EpollEvent{Events: unix.POLLIN | unix.POLLHUP, Fd: int32(fd)})
	if err != nil {
		return err
	}
	me.lock.Lock()
	defer me.lock.Unlock()
	me.connections[fd] = conn
	if len(me.connections)%100 == 0 {
		log.Printf("Concurrent connections: %v", len(me.connections))
	}
	return nil
}

func (me *epoll) Remove(conn net.Conn) error {
	fd := websocketFD(conn)
	err := unix.EpollCtl(me.fd, syscall.EPOLL_CTL_DEL, fd, nil)
	if err != nil {
		return err
	}
	me.lock.Lock()
	defer me.lock.Unlock()
	delete(me.connections, fd)
	if len(me.connections)%100 == 0 {
		log.Printf("Concurrent connections: %v", len(me.connections))
	}
	return nil
}

func (me *epoll) Wait() ([]net.Conn, error) {
	events := make([]unix.EpollEvent, 100)
	n, err := unix.EpollWait(me.fd, events, 100)
	if err != nil {
		return nil, err
	}
	me.lock.RLock()
	defer me.lock.RUnlock()
	var connections []net.Conn
	for i := 0; i < n; i++ {
		conn := me.connections[int(events[i].Fd)]
		connections = append(connections, conn)
	}
	return connections, nil
}

func websocketFD(conn net.Conn) int {
	tcpConn := reflect.Indirect(reflect.ValueOf(conn)).FieldByName("conn")
	fdVal := tcpConn.FieldByName("fd")
	pfdVal := reflect.Indirect(fdVal).FieldByName("pfd")

	return int(pfdVal.FieldByName("Sysfd").Int())
}
