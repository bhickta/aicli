package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
)

type Ollama struct {
	cfg    config.ProviderConfig
	client *http.Client
}

func New(cfg config.ProviderConfig, client *http.Client) *Ollama {
	return &Ollama{cfg: cfg, client: client}
}

func (p *Ollama) ID() string { return p.cfg.ID }

func (p *Ollama) Health(ctx context.Context) error {
	_, err := p.ListModels(ctx)
	return err
}

func (p *Ollama) ListModels(ctx context.Context) ([]provider.Model, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(p.cfg.BaseURL, "/")+"/api/tags", nil)
	if err != nil {
		return nil, err
	}
	res, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, fmt.Errorf("list models: %s", res.Status)
	}

	var payload struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	models := make([]provider.Model, 0, len(payload.Models))
	for _, item := range payload.Models {
		if item.Name != "" {
			models = append(models, provider.Model{ID: item.Name, Name: item.Name})
		}
	}
	return models, nil
}

func (p *Ollama) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	model := p.chatModel(req.Model)
	if model == "" {
		return provider.ChatResponse{}, errors.New("model is required")
	}
	body := ollamaChatBody(model, req, false)
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
		return provider.ChatResponse{}, fmt.Errorf("chat: %s: %s", res.Status, strings.TrimSpace(string(msg)))
	}

	var payload struct {
		Message provider.Message `json:"message"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return provider.ChatResponse{}, err
	}
	return provider.ChatResponse{Content: payload.Message.Content}, nil
}

func (p *Ollama) chatModel(model string) string {
	if model != "" {
		return model
	}
	return p.cfg.Model
}

func (p *Ollama) chatRequest(ctx context.Context, data []byte) (*http.Request, error) {
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(p.cfg.BaseURL, "/")+"/api/chat",
		bytes.NewReader(data),
	)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	return httpReq, nil
}

func ollamaChatBody(model string, req provider.ChatRequest, stream bool) map[string]any {
	return map[string]any{
		"model":    model,
		"messages": req.Messages,
		"stream":   stream,
		"options": map[string]any{
			"temperature": req.Temperature,
		},
	}
}
