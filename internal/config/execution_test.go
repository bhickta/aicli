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

func TestNormalizeExecutionProfilesDefaultsToOrderedSelection(t *testing.T) {
	profiles := NormalizeExecutionProfiles([]ExecutionProfile{{
		ID: "text", Capability: CapabilityText, Enabled: true,
		Targets: []ExecutionTarget{{ProviderID: "one", Priority: 20}, {ProviderID: "two", Priority: 10}},
	}})

	if profiles[0].SelectionStrategy != SelectionOrdered {
		t.Fatalf("selection strategy = %q, want %q", profiles[0].SelectionStrategy, SelectionOrdered)
	}
	if profiles[0].Targets[0].ProviderID != "two" {
		t.Fatalf("first provider = %q, want priority-sorted two", profiles[0].Targets[0].ProviderID)
	}
}
