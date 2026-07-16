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
		for _, target := range profile.Targets {
			if target.ProviderID == "" || !providerIDs[target.ProviderID] {
				return fmt.Errorf("profile %s references unknown provider %q", profile.ID, target.ProviderID)
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
