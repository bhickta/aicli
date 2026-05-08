package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCreatesDefaultSettings(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "settings.json")
	settings, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if settings.DefaultProvider != "lmstudio" {
		t.Fatalf("DefaultProvider = %q, want lmstudio", settings.DefaultProvider)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("settings file was not created: %v", err)
	}
}

func TestLoadRejectsInvalidJSON(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("Load() error = nil, want invalid JSON error")
	}
}
