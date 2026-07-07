package config

import "testing"

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
