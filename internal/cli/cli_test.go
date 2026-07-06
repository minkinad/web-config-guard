package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
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
