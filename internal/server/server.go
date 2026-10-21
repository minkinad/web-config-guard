package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/minkin/web-config-guard/internal/config"
	"github.com/minkin/web-config-guard/internal/runner"
)

const MaxConcurrentChecks = 4

type Server struct {
	Runner    runner.Runner
	Logger    *slog.Logger
	requestID atomic.Uint64
	checks    chan struct{}
}

func New(run runner.Runner, logger ...*slog.Logger) *Server {
	s := &Server{Runner: run, Logger: slog.Default(), checks: make(chan struct{}, MaxConcurrentChecks)}
	if len(logger) > 0 && logger[0] != nil {
		s.Logger = logger[0]
	}
	return s
}

func (server *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", server.health)
	mux.HandleFunc("GET /health/live", server.health)
	mux.HandleFunc("GET /health/ready", server.health)
	mux.HandleFunc("POST /v1/check", server.check)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		id := r.Header.Get("X-Request-ID")
		if id == "" || len(id) > 128 || strings.IndexFunc(id, invalidRequestIDRune) >= 0 {
			id = strconv.FormatUint(server.requestID.Add(1), 10)
		}
		w.Header().Set("X-Request-ID", id)
		mux.ServeHTTP(w, r)
		route := "unmatched"
		switch r.URL.Path {
		case "/v1/check", "/healthz", "/health/live", "/health/ready":
			route = r.URL.Path
		}
		server.Logger.Info("request", "request_id", id, "method", r.Method, "route", route, "duration", time.Since(start))
	})
}

func invalidRequestIDRune(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
		return false
	default:
		return true
	}
}

func (server *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (server *Server) check(w http.ResponseWriter, request *http.Request) {
	defer func() { _ = request.Body.Close() }()
	if request.ContentLength > config.MaxDocumentBytes {
		writeError(w, http.StatusRequestEntityTooLarge, nil)
		return
	}
	select {
	case server.checks <- struct{}{}:
		defer func() { <-server.checks }()
	default:
		w.Header().Set("Retry-After", "1")
		writeError(w, http.StatusServiceUnavailable, nil)
		return
	}

	data, err := io.ReadAll(http.MaxBytesReader(w, request.Body, config.MaxDocumentBytes))
	if err != nil {
		var limit *http.MaxBytesError
		if errors.As(err, &limit) {
			writeError(w, http.StatusRequestEntityTooLarge, nil)
			return
		}
		if request.Context().Err() != nil {
			return
		}
		writeError(w, http.StatusBadRequest, err)
		return
	}

	sourceName := request.URL.Query().Get("filename")
	if sourceName == "" {
		sourceName = "config.yaml"
	}

	result, err := server.Runner.CheckBytesContext(request.Context(), data, sourceName)
	if err != nil {
		if errors.Is(err, context.Canceled) || request.Context().Err() != nil {
			return
		}
		if errors.Is(err, context.DeadlineExceeded) {
			writeError(w, http.StatusRequestTimeout, nil)
			return
		}
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func writeError(w http.ResponseWriter, status int, err error) {
	if err == nil {
		err = errors.New(http.StatusText(status))
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
