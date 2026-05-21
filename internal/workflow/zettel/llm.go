package zettel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/bhickta/aicli/internal/provider"
)

type embedder interface {
	Embeddings(context.Context, provider.EmbeddingRequest) (provider.EmbeddingResponse, error)
}

func chatJSON[T any](ctx context.Context, p provider.Provider, model string, messages []provider.Message) (T, error) {
	var out T
	if p == nil {
		return out, errors.New("provider is required")
	}
	res, err := p.Chat(ctx, provider.ChatRequest{
		Model:       model,
		Messages:    messages,
		Temperature: 0,
	})
	if err != nil {
		return out, err
	}
	content, err := extractJSONObject(res.Content)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return out, fmt.Errorf("parse model json: %w", err)
	}
	return out, nil
}

func extractJSONObject(text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", errors.New("model returned an empty response")
	}
	if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```json")
		text = strings.TrimPrefix(text, "```")
		text = strings.TrimSuffix(text, "```")
		text = strings.TrimSpace(text)
	}
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end < start {
		return "", errors.New("model response did not contain a json object")
	}
	return text[start : end+1], nil
}
