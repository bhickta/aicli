package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
)

type OpenAICompatible struct {
	cfg    config.ProviderConfig
	client *http.Client
}

func NewCompatible(cfg config.ProviderConfig, client *http.Client) *OpenAICompatible {
	return &OpenAICompatible{cfg: cfg, client: client}
}

func (p *OpenAICompatible) ID() string { return p.cfg.ID }

func (p *OpenAICompatible) Health(ctx context.Context) error {
	_, err := p.ListModels(ctx)
	return err
}

func (p *OpenAICompatible) ListModels(ctx context.Context) ([]provider.Model, error) {
	modelsURL := openAIURL(p.cfg.BaseURL, "/models")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsURL, nil)
	if err != nil {
		return nil, err
	}
	p.authorize(req)

	res, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list models %s: %w", modelsURL, err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return nil, fmt.Errorf("list models %s: %s: %s", modelsURL, res.Status, strings.TrimSpace(string(msg)))
	}

	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	models := make([]provider.Model, 0, len(payload.Data))
	for _, item := range payload.Data {
		if item.ID != "" && p.allowsModel(item.ID) {
			models = append(models, provider.Model{ID: item.ID, Name: item.ID})
		}
	}
	return models, nil
}

func (p *OpenAICompatible) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	if p.usesResponsesAPI() {
		return p.responsesChat(ctx, req)
	}
	model := p.chatModel(req.Model)
	if model == "" {
		return provider.ChatResponse{}, errors.New("model is required")
	}
	body := map[string]any{
		"model":       model,
		"messages":    req.Messages,
		"temperature": req.Temperature,
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	return p.chatRaw(ctx, body)
}

func (p *OpenAICompatible) chatRaw(ctx context.Context, body map[string]any) (provider.ChatResponse, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return provider.ChatResponse{}, err
	}
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		openAIURL(p.cfg.BaseURL, "/chat/completions"),
		bytes.NewReader(data),
	)
	if err != nil {
		return provider.ChatResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	p.authorize(httpReq)

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
		Choices []struct {
			Message provider.Message `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return provider.ChatResponse{}, err
	}
	if len(payload.Choices) == 0 {
		return provider.ChatResponse{}, errors.New("chat response has no choices")
	}
	return provider.ChatResponse{Content: payload.Choices[0].Message.Content}, nil
}

func (p *OpenAICompatible) chatModel(model string) string {
	if model != "" {
		return model
	}
	return p.cfg.Model
}

func (p *OpenAICompatible) authorize(req *http.Request) {
	if apiKey := p.resolvedAPIKey(); apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	for key, value := range p.cfg.Headers {
		req.Header.Set(key, value)
	}
}

func (p *OpenAICompatible) resolvedAPIKey() string {
	if p.cfg.APIKey != "" {
		return p.cfg.APIKey
	}
	if p.cfg.APIKeyEnv == "" {
		return ""
	}
	return os.Getenv(p.cfg.APIKeyEnv)
}

func (p *OpenAICompatible) allowsModel(id string) bool {
	filter := strings.TrimSpace(p.cfg.ModelFilter)
	if filter == "" {
		return true
	}
	id = strings.ToLower(id)
	for _, term := range strings.FieldsFunc(filter, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n'
	}) {
		term = strings.ToLower(strings.TrimSpace(term))
		if term != "" && strings.Contains(id, term) {
			return true
		}
	}
	return false
}

func openAIURL(baseURL string, path string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	if strings.HasSuffix(baseURL, "/v1") {
		return baseURL + path
	}
	return baseURL + "/v1" + path
}
