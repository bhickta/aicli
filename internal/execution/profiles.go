package execution

import (
	"fmt"
	"strings"

	"github.com/bhickta/aicli/internal/config"
)

func ValidateProfiles(profiles []config.ExecutionProfile, providerIDs map[string]bool) error {
	seen := make(map[string]bool, len(profiles))
	for _, profile := range profiles {
		if profile.ID == "" || seen[profile.ID] {
			return fmt.Errorf("execution profile ids must be non-empty and unique: %q", profile.ID)
		}
		seen[profile.ID] = true
		if !validCapability(profile.Capability) {
			return fmt.Errorf("unsupported capability %q for profile %s", profile.Capability, profile.ID)
		}
		if profile.SelectionStrategy != config.SelectionOrdered && profile.SelectionStrategy != config.SelectionRoundRobin {
			return fmt.Errorf("unsupported selection strategy %q for profile %s", profile.SelectionStrategy, profile.ID)
		}
		for _, target := range profile.Targets {
			if target.ProviderID == "" || !providerIDs[target.ProviderID] {
				return fmt.Errorf("profile %s references unknown provider %q", profile.ID, target.ProviderID)
			}
			if target.MaxConcurrency < 0 {
				return fmt.Errorf("profile %s target %s has a negative max concurrency", profile.ID, target.ProviderID)
			}
			if target.RateLimit.RequestsPerMinute < 0 || target.RateLimit.TokensPerMinute < 0 || target.RateLimit.RequestsPerDay < 0 {
				return fmt.Errorf("profile %s target %s has a negative rate limit", profile.ID, target.ProviderID)
			}
		}
	}
	return nil
}

func validCapability(value string) bool {
	switch strings.ToLower(value) {
	case config.CapabilityText, config.CapabilityStructured, config.CapabilityVision,
		config.CapabilityEmbedding, config.CapabilityRerank, config.CapabilityOCR:
		return true
	default:
		return false
	}
}
