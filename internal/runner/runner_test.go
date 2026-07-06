package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckPathChecksDirectoryRecursively(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "nested", "config.yaml"), []byte("storage:\n  digest-algorithm: MD5\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("digest-algorithm: MD5\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := New().CheckPath(context.Background(), root)
	if err != nil {
		t.Fatalf("CheckPath() error = %v", err)
	}
	if len(result.Problems) != 1 {
		t.Fatalf("CheckPath() found %d problems, want 1: %#v", len(result.Problems), result.Problems)
	}
	if result.Problems[0].File == "" {
		t.Fatalf("problem file is empty: %#v", result.Problems[0])
	}
}

func TestCheckBytesParsesAndChecksConfig(t *testing.T) {
	result, err := New().CheckBytes([]byte(`{"log":{"level":"debug"}}`), "config.json")
	if err != nil {
		t.Fatalf("CheckBytes() error = %v", err)
	}
	if len(result.Problems) != 1 {
		t.Fatalf("CheckBytes() found %d problems, want 1: %#v", len(result.Problems), result.Problems)
	}
}
