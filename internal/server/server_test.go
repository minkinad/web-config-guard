package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/minkin/web-config-guard/internal/config"
	"github.com/minkin/web-config-guard/internal/guard"
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

func TestCheckEndpointRejectsOversizedBody(t *testing.T) {
	for _, tc := range []struct {
		name          string
		contentLength int64
	}{
		{name: "known length", contentLength: config.MaxDocumentBytes + 1},
		{name: "streamed length", contentLength: -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/v1/check", strings.NewReader(strings.Repeat("x", config.MaxDocumentBytes+1)))
			request.ContentLength = tc.contentLength
			response := httptest.NewRecorder()
			New(runner.New()).Handler().ServeHTTP(response, request)
			if response.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("status = %d, want 413", response.Code)
			}
			if !strings.Contains(response.Header().Get("Content-Type"), "application/json") {
				t.Fatalf("Content-Type = %q", response.Header().Get("Content-Type"))
			}
		})
	}
}

func TestCheckEndpointRejectsComplexDocument(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/v1/check?filename=config.json", strings.NewReader(strings.Repeat("[", config.MaxDocumentDepth+1)+"0"+strings.Repeat("]", config.MaxDocumentDepth+1)))
	response := httptest.NewRecorder()
	New(runner.New()).Handler().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "depth") {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

type gatedRule struct {
	entered  chan<- struct{}
	release  <-chan struct{}
	canceled chan<- struct{}
}

func (gatedRule) ID() string                { return "gate" }
func (gatedRule) Check(any) []guard.Problem { return nil }
func (rule gatedRule) CheckContext(ctx context.Context, _ any) ([]guard.Problem, error) {
	rule.entered <- struct{}{}
	select {
	case <-ctx.Done():
		if rule.canceled != nil {
			rule.canceled <- struct{}{}
		}
		return nil, ctx.Err()
	case <-rule.release:
		return nil, nil
	}
}

func TestCheckEndpointOverloadAndRecovery(t *testing.T) {
	entered := make(chan struct{}, MaxConcurrentChecks)
	release := make(chan struct{})
	run := runner.Runner{Checker: guard.NewChecker([]guard.Rule{gatedRule{entered: entered, release: release}})}
	httpServer := httptest.NewServer(New(run).Handler())
	defer httpServer.Close()
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	client := &http.Client{Timeout: 3 * time.Second}
	responses := make(chan error, MaxConcurrentChecks)
	for range MaxConcurrentChecks {
		go func() {
			response, err := client.Post(httpServer.URL+"/v1/check", "application/yaml", strings.NewReader("a: b\n"))
			if err == nil {
				_, _ = io.Copy(io.Discard, response.Body)
				_ = response.Body.Close()
				if response.StatusCode != http.StatusOK {
					err = fmt.Errorf("status = %d", response.StatusCode)
				}
			}
			responses <- err
		}()
	}
	for range MaxConcurrentChecks {
		select {
		case <-entered:
		case <-time.After(3 * time.Second):
			t.Fatal("checks did not enter the gate")
		}
	}
	response, err := client.Post(httpServer.URL+"/v1/check", "application/yaml", strings.NewReader("a: b\n"))
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable || response.Header.Get("Retry-After") != "1" {
		t.Fatalf("overload status = %d, retry-after = %q", response.StatusCode, response.Header.Get("Retry-After"))
	}
	health, err := client.Get(httpServer.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	_ = health.Body.Close()
	if health.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d", health.StatusCode)
	}
	close(release)
	for range MaxConcurrentChecks {
		if err := <-responses; err != nil {
			t.Fatal(err)
		}
	}
	response, err = client.Post(httpServer.URL+"/v1/check", "application/yaml", strings.NewReader("a: b\n"))
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("recovery status = %d", response.StatusCode)
	}
}

func TestCheckEndpointCancellationStopsCheck(t *testing.T) {
	entered := make(chan struct{}, 1)
	canceled := make(chan struct{}, 1)
	release := make(chan struct{})
	run := runner.Runner{Checker: guard.NewChecker([]guard.Rule{gatedRule{entered: entered, release: release, canceled: canceled}})}
	httpServer := httptest.NewServer(New(run).Handler())
	defer httpServer.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, httpServer.URL+"/v1/check", strings.NewReader("a: b\n"))
	if err != nil {
		t.Fatal(err)
	}
	requestDone := make(chan struct{})
	go func() {
		response, err := http.DefaultClient.Do(request)
		if err == nil {
			_ = response.Body.Close()
		}
		close(requestDone)
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("check did not start")
	}
	cancel()
	select {
	case <-canceled:
	case <-time.After(3 * time.Second):
		t.Fatal("check did not observe cancellation")
	}
	select {
	case <-requestDone:
	case <-time.After(3 * time.Second):
		t.Fatal("request did not finish")
	}
}
