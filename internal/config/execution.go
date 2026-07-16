package config

import (
	"sort"
	"strings"
)

const (
	CapabilityText       = "text"
	CapabilityStructured = "structured"
	CapabilityVision     = "vision"
	CapabilityEmbedding  = "embedding"
	CapabilityRerank     = "rerank"
	CapabilityOCR        = "ocr"
)

type ExecutionProfile struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Capability      string            `json:"capability"`
	Enabled         bool              `json:"enabled"`
	MaxConcurrency  int               `json:"max_concurrency"`
	TimeoutSeconds  int               `json:"timeout_seconds"`
	CooldownSeconds int               `json:"cooldown_seconds"`
	Targets         []ExecutionTarget `json:"targets"`
}

type ExecutionTarget struct {
	ProviderID           string  `json:"provider_id"`
	Model                string  `json:"model"`
	Priority             int     `json:"priority"`
	Enabled              bool    `json:"enabled"`
	InputCostPerMillion  float64 `json:"input_cost_per_million"`
	OutputCostPerMillion float64 `json:"output_cost_per_million"`
}

func DefaultExecutionProfiles() []ExecutionProfile {
	return []ExecutionProfile{
		defaultProfile("default-text", "Default Text", CapabilityText, "lms", ""),
		defaultProfile("default-structured", "Default Structured", CapabilityStructured, "lms", ""),
		defaultProfile("default-vision", "Default Vision", CapabilityVision, "lms", ""),
		defaultProfile("default-embedding", "Default Embedding", CapabilityEmbedding, "lms", "text-embedding-nomic-embed-text-v1.5"),
		defaultProfile("default-rerank", "Default Reranker", CapabilityRerank, "vllm", "Qwen/Qwen3-Reranker-4B"),
		defaultProfile("default-ocr", "Default OCR", CapabilityOCR, "vllm", "baidu/Unlimited-OCR"),
	}
}

func NormalizeExecutionProfiles(profiles []ExecutionProfile) []ExecutionProfile {
	if profiles == nil {
		profiles = DefaultExecutionProfiles()
	}
	for index := range profiles {
		profile := &profiles[index]
		profile.ID = strings.TrimSpace(profile.ID)
		profile.Name = strings.TrimSpace(profile.Name)
		profile.Capability = strings.ToLower(strings.TrimSpace(profile.Capability))
		if profile.MaxConcurrency <= 0 {
			profile.MaxConcurrency = 1
		}
		if profile.TimeoutSeconds <= 0 {
			profile.TimeoutSeconds = 120
		}
		if profile.CooldownSeconds <= 0 {
			profile.CooldownSeconds = 14400
		}
		sort.SliceStable(profile.Targets, func(i, j int) bool {
			return profile.Targets[i].Priority < profile.Targets[j].Priority
		})
	}
	return profiles
}

func defaultProfile(id, name, capability, providerID, model string) ExecutionProfile {
	return ExecutionProfile{
		ID: id, Name: name, Capability: capability, Enabled: true,
		MaxConcurrency: 1, TimeoutSeconds: 120, CooldownSeconds: 14400,
		Targets: []ExecutionTarget{{ProviderID: providerID, Model: model, Priority: 10, Enabled: true}},
	}
}
