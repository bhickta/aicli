package server

import (
	"errors"
	"strings"

	"github.com/bhickta/aicli/internal/config"
)

func sanitizedProviders(providers []config.ProviderConfig) []config.ProviderConfig {
	result := make([]config.ProviderConfig, len(providers))
	copy(result, providers)
	for index := range result {
		result[index].APIKey = ""
		result[index].Headers = maskedHeaders(result[index].Headers)
	}
	return result
}

func sanitizedSettings(settings config.Settings) config.Settings {
	settings.Providers = sanitizedProviders(settings.Providers)
	return settings
}

func preserveProviderSecrets(incoming, existing []config.ProviderConfig) []config.ProviderConfig {
	byID := make(map[string]config.ProviderConfig, len(existing))
	for _, provider := range existing {
		byID[provider.ID] = provider
	}
	for index := range incoming {
		current := byID[incoming[index].ID]
		if incoming[index].APIKey == "" {
			incoming[index].APIKey = current.APIKey
		}
		if incoming[index].APIKeyEnv == "" {
			incoming[index].APIKeyEnv = current.APIKeyEnv
		}
		incoming[index].Headers = preserveHeaderSecrets(incoming[index].Headers, current.Headers)
	}
	return incoming
}

func maskedHeaders(headers map[string]string) map[string]string {
	masked := make(map[string]string, len(headers))
	for key := range headers {
		masked[key] = ""
	}
	return masked
}

func preserveHeaderSecrets(incoming, current map[string]string) map[string]string {
	if incoming == nil {
		incoming = make(map[string]string, len(current))
	}
	for key, value := range current {
		if incoming[key] == "" {
			incoming[key] = value
		}
	}
	return incoming
}

func validateProviders(providers []config.ProviderConfig) error {
	seen := make(map[string]bool, len(providers))
	for _, provider := range providers {
		id := strings.TrimSpace(provider.ID)
		if id == "" || seen[id] {
			return errors.New("provider ids must be non-empty and unique")
		}
		seen[id] = true
	}
	return nil
}

func configuredProviderIDs(providers []config.ProviderConfig) map[string]bool {
	result := make(map[string]bool, len(providers))
	for _, provider := range providers {
		result[provider.ID] = true
	}
	return result
}
