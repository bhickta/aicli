package model

type APICallUsage struct {
	Total      int                    `json:"total"`
	Chat       int                    `json:"chat"`
	Embeddings int                    `json:"embeddings"`
	Vision     int                    `json:"vision"`
	Stream     int                    `json:"stream"`
	Providers  []ProviderAPICallUsage `json:"providers,omitempty"`
}

type ProviderAPICallUsage struct {
	ProviderID string `json:"provider_id"`
	Total      int    `json:"total"`
	Chat       int    `json:"chat"`
	Embeddings int    `json:"embeddings"`
	Vision     int    `json:"vision"`
	Stream     int    `json:"stream"`
}
