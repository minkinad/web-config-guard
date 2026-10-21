package server

import (
	"net"
	"sync"
)

const MaxConnections = 64

type limitedListener struct {
	net.Listener
	slots chan struct{}
	done  chan struct{}
	once  sync.Once
}

// LimitConnections bounds the number of accepted connections, including idle
// keep-alive and clients still sending headers.
func LimitConnections(listener net.Listener, limit int) net.Listener {
	return &limitedListener{
		Listener: listener,
		slots:    make(chan struct{}, limit),
		done:     make(chan struct{}),
	}
}

func (listener *limitedListener) Accept() (net.Conn, error) {
	select {
	case listener.slots <- struct{}{}:
	case <-listener.done:
		return nil, net.ErrClosed
	}
	conn, err := listener.Listener.Accept()
	if err != nil {
		<-listener.slots
		return nil, err
	}
	return &limitedConn{Conn: conn, release: func() { <-listener.slots }}, nil
}

func (listener *limitedListener) Close() error {
	listener.once.Do(func() { close(listener.done) })
	return listener.Listener.Close()
}

type limitedConn struct {
	net.Conn
	release func()
	once    sync.Once
}

func (conn *limitedConn) Close() error {
	err := conn.Conn.Close()
	conn.once.Do(conn.release)
	return err
}
