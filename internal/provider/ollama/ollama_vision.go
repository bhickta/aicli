package ollama

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/bhickta/aicli/internal/provider"
)

func (p *Ollama) Vision(ctx context.Context, req provider.VisionRequest) (provider.ChatResponse, error) {
	model := p.chatModel(req.Model)
	if model == "" {
		return provider.ChatResponse{}, errors.New("model is required")
	}
	if len(req.Image) == 0 {
		return provider.ChatResponse{}, errors.New("image is required")
	}

	body := map[string]any{
		"model": model,
		"messages": []map[string]any{
			{
				"role":    "user",
				"content": req.Prompt,
				"images":  []string{base64.StdEncoding.EncodeToString(req.Image)},
			},
		},
		"stream": false,
		"options": map[string]any{
			"temperature": req.Temperature,
		},
	}
	data, err := json.Marshal(body)
	if err != nil {
		return provider.ChatResponse{}, err
	}
	httpReq, err := p.chatRequest(ctx, data)
	if err != nil {
		return provider.ChatResponse{}, err
	}

	res, err := p.client.Do(httpReq)
	if err != nil {
		return provider.ChatResponse{}, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return provider.ChatResponse{}, fmt.Errorf("vision: %s: %s", res.Status, strings.TrimSpace(string(msg)))
	}

	var payload struct {
		Message provider.Message `json:"message"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return provider.ChatResponse{}, err
	}
	return provider.ChatResponse{Content: payload.Message.Content}, nil
}
