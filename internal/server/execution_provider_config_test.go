package server

import (
	"testing"

	"github.com/bhickta/aicli/internal/config"
)

func TestSanitizedProvidersMasksKeysAndHeadersWithoutMutatingSource(t *testing.T) {
	source := []config.ProviderConfig{{
		ID: "private", APIKey: "secret", Headers: map[string]string{"Authorization": "Bearer secret"},
	}}
	result := sanitizedProviders(source)
	if result[0].APIKey != "" || result[0].Headers["Authorization"] != "" {
		t.Fatalf("provider secrets were exposed: %#v", result[0])
	}
	if source[0].APIKey != "secret" || source[0].Headers["Authorization"] != "Bearer secret" {
		t.Fatalf("source provider was mutated: %#v", source[0])
	}
}

func TestPreserveProviderSecretsRetainsMaskedValues(t *testing.T) {
	existing := []config.ProviderConfig{{
		ID: "private", APIKey: "secret", APIKeyEnv: "TOKEN", Headers: map[string]string{"Authorization": "Bearer secret"},
	}}
	incoming := []config.ProviderConfig{{ID: "private", Headers: map[string]string{"Authorization": ""}}}
	result := preserveProviderSecrets(incoming, existing)
	if result[0].APIKey != "secret" || result[0].APIKeyEnv != "TOKEN" || result[0].Headers["Authorization"] != "Bearer secret" {
		t.Fatalf("provider secrets were not preserved: %#v", result[0])
	}
}
