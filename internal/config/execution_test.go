package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultExecutionProfilesCoverEveryCapability(t *testing.T) {
	want := map[string]bool{
		CapabilityText: true, CapabilityStructured: true, CapabilityVision: true,
		CapabilityEmbedding: true, CapabilityRerank: true, CapabilityOCR: true,
	}
	for _, profile := range DefaultExecutionProfiles() {
		delete(want, profile.Capability)
		if !profile.Enabled || len(profile.Targets) == 0 {
			t.Fatalf("default profile is not executable: %#v", profile)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing default capabilities: %#v", want)
	}
}

func TestSaveRestrictsExistingConfigPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, DefaultSettings()); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("config mode = %o, want 600", info.Mode().Perm())
	}
}
