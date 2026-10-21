package config

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestParseJSON(t *testing.T) {
	value, err := Parse([]byte(`{"log":{"level":"debug"},"port":8080}`), "config.json")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	root, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("Parse() returned %T, want map[string]any", value)
	}
	if _, ok := root["log"].(map[string]any); !ok {
		t.Fatalf("log was not normalized to map[string]any: %#v", root["log"])
	}
}

func TestParseJSONPreservesLargeNumber(t *testing.T) {
	value, err := Parse([]byte(`{"limit":1e999999}`), "config.json")
	if err != nil {
		t.Fatal(err)
	}
	if got := value.(map[string]any)["limit"]; fmt.Sprint(got) != "1e999999" {
		t.Fatalf("limit = %v, want original JSON number", got)
	}
}

func TestParseYAML(t *testing.T) {
	value, err := Parse([]byte("storage:\n  digest-algorithm: MD5\n"), "config.yaml")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	root := value.(map[string]any)
	storage := root["storage"].(map[string]any)
	if storage["digest-algorithm"] != "MD5" {
		t.Fatalf("digest-algorithm = %#v, want MD5", storage["digest-algorithm"])
	}
}

func TestParseEmptyConfig(t *testing.T) {
	if _, err := Parse([]byte("   \n"), "config.yaml"); err == nil {
		t.Fatal("Parse() error = nil, want error")
	}
}

func TestParseJSONRejectsTrailingDocument(t *testing.T) {
	if _, err := Parse([]byte(`{"log":{"level":"info"}} {"debug":true}`), "config.json"); err == nil {
		t.Fatal("Parse() error = nil, want error")
	}
}

func TestParseYAMLRejectsMultipleDocuments(t *testing.T) {
	data := []byte("log:\n  level: info\n---\ndebug: true\n")
	if _, err := Parse(data, "config.yaml"); err == nil {
		t.Fatal("Parse() error = nil, want error")
	}
}

func TestParseRejectsOversizedDocument(t *testing.T) {
	_, err := Parse([]byte(strings.Repeat("x", MaxDocumentBytes+1)), "config.yaml")
	if err == nil || !strings.Contains(err.Error(), "10 MiB") {
		t.Fatalf("Parse() error = %v, want size limit", err)
	}
}

func TestParseDocumentComplexity(t *testing.T) {
	deepJSON := strings.Repeat("[", MaxDocumentDepth+1) + "0" + strings.Repeat("]", MaxDocumentDepth+1)
	var deepYAML strings.Builder
	for i := 0; i < MaxDocumentDepth+1; i++ {
		deepYAML.WriteString(strings.Repeat("  ", i) + "-\n")
	}
	deepYAML.WriteString(strings.Repeat("  ", MaxDocumentDepth+1) + "value\n")
	cases := []struct {
		name, filename, data, want string
	}{
		{"JSON depth", "config.json", deepJSON, "depth"},
		{"YAML depth", "config.yaml", deepYAML.String(), "depth"},
		{"JSON nodes", "config.json", "[" + strings.Repeat("0,", MaxDocumentNodes) + "0]", "nodes"},
		{"YAML nodes", "config.yaml", strings.Repeat("- value\n", MaxDocumentNodes), "nodes"},
		{"YAML alias", "config.yaml", "a: &shared {x: y}\nb: *shared\n", "aliases"},
		{"JSON path", "config.json", `{"` + strings.Repeat("x", MaxPathBytes+1) + `":true}`, "path"},
		{"YAML path", "config.yaml", strings.Repeat("x", MaxPathBytes+1) + ": true\n", "path"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.data), tc.filename)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Parse() error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestParseAcceptsLimitBoundary(t *testing.T) {
	deepJSON := strings.Repeat("[", MaxDocumentDepth) + "0" + strings.Repeat("]", MaxDocumentDepth)
	var deepYAML strings.Builder
	for i := 0; i < MaxDocumentDepth; i++ {
		deepYAML.WriteString(strings.Repeat("  ", i) + "-\n")
	}
	deepYAML.WriteString(strings.Repeat("  ", MaxDocumentDepth) + "value\n")
	for _, tc := range []struct {
		filename, data string
	}{
		{"config.json", deepJSON},
		{"config.yaml", deepYAML.String()},
		{"config.json", `{"` + strings.Repeat("x", MaxPathBytes) + `":true}`},
		{"config.yaml", strings.Repeat("x", MaxPathBytes) + ": true\n"},
	} {
		if _, err := Parse([]byte(tc.data), tc.filename); err != nil {
			t.Errorf("Parse(%q) error = %v", tc.filename, err)
		}
	}
}

func TestParseContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, filename := range []string{"config.json", "config.yaml"} {
		if _, err := ParseContext(ctx, []byte("{}"), filename); !errors.Is(err, context.Canceled) {
			t.Fatalf("ParseContext(%q) error = %v, want context.Canceled", filename, err)
		}
	}
}
