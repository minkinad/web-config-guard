package cli

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestRunStdinReturnsIssueExitCode(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"--stdin"}, strings.NewReader(`{"log":{"level":"debug"}}`), &stdout, &stderr)

	if code != exitIssues {
		t.Fatalf("Run() exit code = %d, want %d; stderr=%s", code, exitIssues, stderr.String())
	}
	if !strings.Contains(stdout.String(), "debug") {
		t.Fatalf("stdout does not contain debug problem: %s", stdout.String())
	}
}

func TestRunSilentReturnsOKWhenProblemsFound(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"--stdin", "--silent"}, strings.NewReader(`{"log":{"level":"debug"}}`), &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("Run() exit code = %d, want %d; stderr=%s", code, exitOK, stderr.String())
	}
}

func TestRunJSONFormat(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"--stdin", "--silent", "--format", "json"}, strings.NewReader(`{"log":{"level":"debug"}}`), &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("Run() exit code = %d, want %d; stderr=%s", code, exitOK, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"problems"`) {
		t.Fatalf("stdout is not JSON result: %s", stdout.String())
	}
}

func TestHTTPServerTimeoutDefaults(t *testing.T) {
	srv := newHTTPServer("127.0.0.1:0", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	if srv.ReadHeaderTimeout != 5*time.Second || srv.ReadTimeout != 15*time.Second || srv.WriteTimeout != 30*time.Second || srv.IdleTimeout != 60*time.Second || srv.MaxHeaderBytes != 16<<10 {
		t.Fatalf("unexpected HTTP timeout or header limits: %+v", srv)
	}
}

func TestHTTPServerClosesSlowHeaders(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := newHTTPServer(listener.Addr().String(), http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	srv.ReadHeaderTimeout = 100 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- serveWithShutdown(ctx, srv, listener) }()
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := io.WriteString(conn, "POST /v1/check HTTP/1.1\r\nHost:"); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, err = io.Copy(io.Discard, conn)
	if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
		t.Fatal("slow header connection was not closed")
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestHTTPServerClosesSlowBody(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		_, _ = io.Copy(io.Discard, request.Body)
		w.WriteHeader(http.StatusOK)
	})
	srv := newHTTPServer(listener.Addr().String(), handler)
	srv.ReadTimeout = 100 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- serveWithShutdown(ctx, srv, listener) }()
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := io.WriteString(conn, "POST /v1/check HTTP/1.1\r\nHost: localhost\r\nContent-Length: 5\r\n\r\nx"); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, err = io.Copy(io.Discard, conn)
	if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
		t.Fatal("slow body connection was not closed")
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestHTTPServerWaitsForActiveRequestOnShutdown(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	release := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusOK)
	})
	srv := newHTTPServer(listener.Addr().String(), handler)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- serveWithShutdown(ctx, srv, listener) }()
	requestDone := make(chan error, 1)
	go func() {
		client := &http.Client{Timeout: 3 * time.Second}
		response, err := client.Get("http://" + listener.Addr().String())
		if err == nil {
			_ = response.Body.Close()
		}
		requestDone <- err
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("request did not start")
	}
	cancel()
	select {
	case err := <-done:
		t.Fatalf("server returned before active request completed: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	select {
	case err := <-requestDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("request did not finish")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not shut down")
	}
}
