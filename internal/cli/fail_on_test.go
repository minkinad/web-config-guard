package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestFailOnThreshold(t *testing.T) {
	cases := []struct {
		threshold string
		want      int
	}{{"low", 1}, {"medium", 0}, {"high", 0}}
	for _, tc := range cases {
		t.Run(tc.threshold, func(t *testing.T) {
			var out, stderr bytes.Buffer
			got := Run(context.Background(), []string{"--stdin", "--fail-on", tc.threshold}, strings.NewReader("log:\n  level: debug\n"), &out, &stderr)
			if got != tc.want {
				t.Fatalf("exit=%d want=%d stderr=%s", got, tc.want, stderr.String())
			}
			if !strings.Contains(out.String(), "debug") {
				t.Fatal("finding missing from output")
			}
		})
	}
}
