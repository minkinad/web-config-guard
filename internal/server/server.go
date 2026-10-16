package server

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/minkin/web-config-guard/internal/runner"
)

type Server struct {
	Runner    runner.Runner
	Logger    *slog.Logger
	requestID atomic.Uint64
}

func New(run runner.Runner, logger ...*slog.Logger) *Server {
	s := &Server{Runner: run, Logger: slog.Default()}
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
		if id == "" || len(id) > 128 || strings.ContainsAny(id, "\r\n") {
			id = strconv.FormatUint(server.requestID.Add(1), 10)
		}
		w.Header().Set("X-Request-ID", id)
		mux.ServeHTTP(w, r)
		server.Logger.Info("request", "request_id", id, "method", r.Method, "path", r.URL.Path, "duration", time.Since(start))
	})
}

func (server *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (server *Server) check(w http.ResponseWriter, request *http.Request) {
	defer func() { _ = request.Body.Close() }()

	data, err := io.ReadAll(http.MaxBytesReader(w, request.Body, 10<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	sourceName := request.URL.Query().Get("filename")
	if sourceName == "" {
		sourceName = "config.yaml"
	}

	result, err := server.Runner.CheckBytes(data, sourceName)
	if err != nil {
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
