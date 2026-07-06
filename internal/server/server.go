package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/minkin/web-config-guard/internal/runner"
)

type Server struct {
	Runner runner.Runner
}

func New(run runner.Runner) Server {
	return Server{Runner: run}
}

func (server Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", server.health)
	mux.HandleFunc("POST /v1/check", server.check)
	return mux
}

func (server Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (server Server) check(w http.ResponseWriter, request *http.Request) {
	defer request.Body.Close()

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
