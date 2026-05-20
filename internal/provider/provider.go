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
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`
}

type ChatResponse struct {
	Content string `json:"content"`
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
