package server

import (
	"net"
	"testing"
	"time"
)

func TestLimitConnectionsReleasesClosedConnection(t *testing.T) {
	base, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	listener := LimitConnections(base, 1)
	defer func() { _ = listener.Close() }()
	clientOne, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = clientOne.Close() }()
	serverOne, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	clientTwo, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = clientTwo.Close() }()
	accepted := make(chan net.Conn, 1)
	acceptErr := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			acceptErr <- err
			return
		}
		accepted <- conn
	}()
	select {
	case conn := <-accepted:
		_ = conn.Close()
		t.Fatal("second connection accepted while slot was occupied")
	case err := <-acceptErr:
		t.Fatal(err)
	case <-time.After(50 * time.Millisecond):
	}
	if err := serverOne.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case conn := <-accepted:
		_ = conn.Close()
	case err := <-acceptErr:
		t.Fatal(err)
	case <-time.After(2 * time.Second):
		t.Fatal("slot was not released")
	}
}

func TestLimitConnectionsCloseUnblocksAccept(t *testing.T) {
	base, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	listener := LimitConnections(base, 1)
	client, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()
	accepted, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = accepted.Close() }()
	done := make(chan error, 1)
	go func() {
		_, err := listener.Accept()
		done <- err
	}()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Accept() error = nil after close")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Accept() remained blocked after close")
	}
}
