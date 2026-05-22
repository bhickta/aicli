package provider

import (
	"context"
)

type Model struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model                string    `json:"model"`
	Messages             []Message `json:"messages"`
	Temperature          float64   `json:"temperature"`
	MaxTokens            int       `json:"max_tokens"`
	ReasoningEffort      string    `json:"reasoning_effort,omitempty"`
	TextVerbosity        string    `json:"text_verbosity,omitempty"`
	PromptCacheKey       string    `json:"prompt_cache_key,omitempty"`
	PromptCacheRetention string    `json:"prompt_cache_retention,omitempty"`
}

type ChatResponse struct {
	Content string      `json:"content"`
	Usage   *TokenUsage `json:"usage,omitempty"`
}

type TokenUsage struct {
	InputTokens           int `json:"input_tokens,omitempty"`
	CachedInputTokens     int `json:"cached_input_tokens,omitempty"`
	OutputTokens          int `json:"output_tokens,omitempty"`
	ReasoningOutputTokens int `json:"reasoning_output_tokens,omitempty"`
	TotalTokens           int `json:"total_tokens,omitempty"`
}

type EmbeddingRequest struct {
	Model  string   `json:"model"`
	Inputs []string `json:"inputs"`
}

type EmbeddingResponse struct {
	Vectors [][]float64 `json:"vectors"`
}

type VisionRequest struct {
	Model       string  `json:"model"`
	Prompt      string  `json:"prompt"`
	Image       []byte  `json:"-"`
	MIMEType    string  `json:"mime_type"`
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
}

type Provider interface {
	ID() string
	Health(ctx context.Context) error
	ListModels(ctx context.Context) ([]Model, error)
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
	ChatStream(ctx context.Context, req ChatRequest, yield func(string) error) error
	Vision(ctx context.Context, req VisionRequest) (ChatResponse, error)
}
