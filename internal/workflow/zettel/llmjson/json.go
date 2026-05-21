package llmjson

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/bhickta/aicli/internal/provider"
)

func Chat[T any](ctx context.Context, p provider.Provider, model string, messages []provider.Message) (T, error) {
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
	candidates, err := extractJSONObjects(res.Content)
	if err != nil {
		return out, err
	}
	var parseErr error
	for _, content := range candidates {
		if err := json.Unmarshal([]byte(content), &out); err == nil {
			return out, nil
		} else {
			parseErr = err
		}
	}
	return out, fmt.Errorf("parse model json: %w", parseErr)
}

func extractJSONObject(text string) (string, error) {
	candidates, err := extractJSONObjects(text)
	if err != nil {
		return "", err
	}
	return candidates[0], nil
}

func extractJSONObjects(text string) ([]string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, errors.New("model returned an empty response")
	}
	if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```json")
		text = strings.TrimPrefix(text, "```")
		text = strings.TrimSuffix(text, "```")
		text = strings.TrimSpace(text)
	}
	candidates := balancedJSONObjects(text)
	if len(candidates) == 0 {
		return nil, errors.New("model response did not contain a json object")
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return len(candidates[i]) > len(candidates[j])
	})
	return candidates, nil
}

func balancedJSONObjects(text string) []string {
	var out []string
	inString := false
	escaped := false
	depth := 0
	start := -1
	for i := 0; i < len(text); i++ {
		ch := text[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch ch {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{':
			if depth == 0 {
				start = i
			}
			depth++
		case '}':
			if depth == 0 {
				continue
			}
			depth--
			if depth == 0 && start >= 0 {
				out = append(out, text[start:i+1])
				start = -1
			}
		}
	}
	return out
}
