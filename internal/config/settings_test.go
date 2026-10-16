package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSettingsEnvironmentPriority(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.yaml")
	if err := os.WriteFile(path, []byte("fail_on: high\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WCG_FAIL_ON", "medium")
	s, err := LoadSettings(path)
	if err != nil {
		t.Fatal(err)
	}
	if s.FailOn != "medium" {
		t.Fatalf("fail_on=%q", s.FailOn)
	}
}

func TestSettingsRejectsMultipleDocuments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.yaml")
	if err := os.WriteFile(path, []byte("fail_on: low\n---\nfail_on: high\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSettings(path); err == nil {
		t.Fatal("expected error")
	}
}
