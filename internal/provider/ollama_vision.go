package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

func (p *Ollama) Vision(ctx context.Context, req VisionRequest) (ChatResponse, error) {
	model := p.chatModel(req.Model)
	if model == "" {
		return ChatResponse{}, errors.New("model is required")
	}
	if len(req.Image) == 0 {
		return ChatResponse{}, errors.New("image is required")
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
		return ChatResponse{}, err
	}
	httpReq, err := p.chatRequest(ctx, data)
	if err != nil {
		return ChatResponse{}, err
	}

	res, err := p.client.Do(httpReq)
	if err != nil {
		return ChatResponse{}, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return ChatResponse{}, fmt.Errorf("vision: %s: %s", res.Status, strings.TrimSpace(string(msg)))
	}

	var payload struct {
		Message Message `json:"message"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return ChatResponse{}, err
	}
	return ChatResponse{Content: payload.Message.Content}, nil
}
