package execution

import "github.com/bhickta/aicli/internal/provider"

type Request struct {
	CorrelationID string             `json:"correlation_id"`
	Profile       string             `json:"profile"`
	Capability    string             `json:"capability"`
	Prompt        string             `json:"prompt"`
	Messages      []provider.Message `json:"messages"`
	Inputs        []string           `json:"inputs"`
	Query         string             `json:"query"`
	Documents     []string           `json:"documents"`
	Images        []Image            `json:"images"`
	Temperature   float64            `json:"temperature"`
	MaxTokens     int                `json:"max_tokens"`
}

type Image struct {
	Name     string `json:"name"`
	Data     string `json:"data"`
	MIMEType string `json:"mime_type"`
}

type Attempt struct {
	ProviderID string `json:"provider_id"`
	Model      string `json:"model"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
}

type Response struct {
	CorrelationID string                  `json:"correlation_id"`
	Profile       string                  `json:"profile"`
	Capability    string                  `json:"capability"`
	ProviderID    string                  `json:"provider_id"`
	Model         string                  `json:"model"`
	Content       string                  `json:"content,omitempty"`
	Vectors       [][]float64             `json:"vectors,omitempty"`
	RerankResults []provider.RerankResult `json:"rerank_results,omitempty"`
	Usage         *provider.TokenUsage    `json:"usage,omitempty"`
	Attempts      []Attempt               `json:"attempts"`
	DurationMS    int64                   `json:"duration_ms"`
	EstimatedCost float64                 `json:"estimated_cost"`
}
