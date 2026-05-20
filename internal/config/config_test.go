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
	if settings.DefaultProvider != "lms" {
		t.Fatalf("DefaultProvider = %q, want lms", settings.DefaultProvider)
	}
	if !hasProvider(settings.Providers, "codex") {
		t.Fatalf("default settings missing codex provider: %#v", settings.Providers)
	}
	if !hasProvider(settings.Providers, "codex-cli") {
		t.Fatalf("default settings missing codex-cli provider: %#v", settings.Providers)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("settings file was not created: %v", err)
	}
}

func TestNormalizeAppendsMissingDefaultProviders(t *testing.T) {
	t.Parallel()

	settings := Normalize(Settings{
		DefaultProvider: "lms",
		Providers: []ProviderConfig{
			{ID: "lms", Type: "openai-compatible", Name: "Custom LMS", BaseURL: "http://example.test/v1"},
		},
	})
	if len(settings.Providers) < 5 {
		t.Fatalf("providers = %#v, want custom provider plus defaults", settings.Providers)
	}
	if settings.Providers[0].Name != "Custom LMS" {
		t.Fatalf("first provider was overwritten: %#v", settings.Providers[0])
	}
	if !hasProvider(settings.Providers, "codex") {
		t.Fatalf("Normalize() missing codex provider: %#v", settings.Providers)
	}
	if !hasProvider(settings.Providers, "codex-cli") {
		t.Fatalf("Normalize() missing codex-cli provider: %#v", settings.Providers)
	}
}

func TestDefaultCodexProviderUsesAPIKeyEnv(t *testing.T) {
	t.Parallel()

	settings := DefaultSettings()
	var codex ProviderConfig
	for _, provider := range settings.Providers {
		if provider.ID == "codex" {
			codex = provider
			break
		}
	}
	if codex.Type != "openai-responses" {
		t.Fatalf("codex type = %q, want openai-responses", codex.Type)
	}
	if codex.APIKey != "" {
		t.Fatalf("codex APIKey = %q, want empty API key in default settings", codex.APIKey)
	}
	if codex.APIKeyEnv != "OPENAI_API_KEY" {
		t.Fatalf("codex APIKeyEnv = %q, want OPENAI_API_KEY", codex.APIKeyEnv)
	}
	if codex.ModelFilter != "codex" {
		t.Fatalf("codex ModelFilter = %q, want codex", codex.ModelFilter)
	}
	if settings.Tools.CodexCLI != "codex" {
		t.Fatalf("CodexCLI = %q, want codex", settings.Tools.CodexCLI)
	}
}

func TestDefaultCodexCLIProviderUsesLocalCLI(t *testing.T) {
	t.Parallel()

	settings := DefaultSettings()
	var codexCLI ProviderConfig
	for _, provider := range settings.Providers {
		if provider.ID == "codex-cli" {
			codexCLI = provider
			break
		}
	}
	if codexCLI.Type != "codex-cli" {
		t.Fatalf("codex-cli type = %q, want codex-cli", codexCLI.Type)
	}
	if codexCLI.Name != "Codex CLI / Pro" {
		t.Fatalf("codex-cli name = %q, want Codex CLI / Pro", codexCLI.Name)
	}
	if codexCLI.Model == "" {
		t.Fatal("codex-cli default model is empty")
	}
}

func TestNormalizeDefaultsCodexCLI(t *testing.T) {
	t.Parallel()

	settings := Normalize(Settings{Tools: ToolConfig{FFmpeg: "ffmpeg"}})
	if settings.Tools.CodexCLI != "codex" {
		t.Fatalf("CodexCLI = %q, want codex", settings.Tools.CodexCLI)
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

func hasProvider(providers []ProviderConfig, id string) bool {
	for _, provider := range providers {
		if provider.ID == id {
			return true
		}
	}
	return false
}
