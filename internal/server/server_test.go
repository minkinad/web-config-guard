package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/minkin/web-config-guard/internal/runner"
)

func TestCheckEndpoint(t *testing.T) {
	handler := New(runner.New()).Handler()
	request := httptest.NewRequest(http.MethodPost, "/v1/check?filename=config.yaml", strings.NewReader("storage:\n  digest-algorithm: MD5\n"))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "weak-algorithm") {
		t.Fatalf("response does not contain weak-algorithm problem: %s", response.Body.String())
	}
}

func TestCheckEndpointRejectsInvalidConfig(t *testing.T) {
	handler := New(runner.New()).Handler()
	request := httptest.NewRequest(http.MethodPost, "/v1/check", strings.NewReader("{invalid"))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusBadRequest, response.Body.String())
	}
}
