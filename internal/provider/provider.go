package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bhickta/aicli/internal/config"
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

type Registry struct {
	providers map[string]Provider
}

func NewRegistry(configs []config.ProviderConfig) *Registry {
	providers := make(map[string]Provider, len(configs))
	client := &http.Client{Timeout: 30 * time.Minute}
	for _, cfg := range configs {
		switch cfg.Type {
		case "ollama":
			providers[cfg.ID] = NewOllama(cfg, client)
		default:
			providers[cfg.ID] = NewOpenAICompatible(cfg, client)
		}
	}
	return &Registry{providers: providers}
}

func (r *Registry) List() []string {
	ids := make([]string, 0, len(r.providers))
	for id := range r.providers {
		ids = append(ids, id)
	}
	return ids
}

func (r *Registry) Get(id string) (Provider, bool) {
	p, ok := r.providers[id]
	return p, ok
}

type OpenAICompatible struct {
	cfg    config.ProviderConfig
	client *http.Client
}

func NewOpenAICompatible(cfg config.ProviderConfig, client *http.Client) *OpenAICompatible {
	return &OpenAICompatible{cfg: cfg, client: client}
}

func (p *OpenAICompatible) ID() string { return p.cfg.ID }

func (p *OpenAICompatible) Health(ctx context.Context) error {
	_, err := p.ListModels(ctx)
	return err
}

func (p *OpenAICompatible) ListModels(ctx context.Context) ([]Model, error) {
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
	models := make([]Model, 0, len(payload.Data))
	for _, item := range payload.Data {
		if item.ID != "" {
			models = append(models, Model{ID: item.ID, Name: item.ID})
		}
	}
	return models, nil
}

func (p *OpenAICompatible) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = p.cfg.Model
	}
	if model == "" {
		return ChatResponse{}, errors.New("model is required")
	}

	body := map[string]any{
		"model":       model,
		"messages":    req.Messages,
		"temperature": req.Temperature,
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	data, err := json.Marshal(body)
	if err != nil {
		return ChatResponse{}, err
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		openAIURL(p.cfg.BaseURL, "/chat/completions"),
		bytes.NewReader(data),
	)
	if err != nil {
		return ChatResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	p.authorize(httpReq)

	res, err := p.client.Do(httpReq)
	if err != nil {
		return ChatResponse{}, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return ChatResponse{}, fmt.Errorf("chat: %s: %s", res.Status, strings.TrimSpace(string(msg)))
	}

	var payload struct {
		Choices []struct {
			Message Message `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return ChatResponse{}, err
	}
	if len(payload.Choices) == 0 {
		return ChatResponse{}, errors.New("chat response has no choices")
	}
	return ChatResponse{Content: payload.Choices[0].Message.Content}, nil
}

func (p *OpenAICompatible) ChatStream(ctx context.Context, req ChatRequest, yield func(string) error) error {
	model := req.Model
	if model == "" {
		model = p.cfg.Model
	}
	if model == "" {
		return errors.New("model is required")
	}
	body := map[string]any{
		"model":       model,
		"messages":    req.Messages,
		"temperature": req.Temperature,
		"stream":      true,
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		openAIURL(p.cfg.BaseURL, "/chat/completions"),
		bytes.NewReader(data),
	)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	p.authorize(httpReq)
	res, err := p.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return fmt.Errorf("chat stream: %s: %s", res.Status, strings.TrimSpace(string(msg)))
	}
	scanner := bufio.NewScanner(res.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if line == "[DONE]" {
			return nil
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			if err := yield(chunk.Choices[0].Delta.Content); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}

func (p *OpenAICompatible) Vision(ctx context.Context, req VisionRequest) (ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = p.cfg.Model
	}
	if model == "" {
		return ChatResponse{}, errors.New("model is required")
	}
	if len(req.Image) == 0 {
		return ChatResponse{}, errors.New("image is required")
	}
	mimeType := req.MIMEType
	if mimeType == "" {
		mimeType = "image/jpeg"
	}

	body := map[string]any{
		"model": model,
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "text", "text": req.Prompt},
					{
						"type": "image_url",
						"image_url": map[string]any{
							"url": "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(req.Image),
						},
					},
				},
			},
		},
		"temperature": req.Temperature,
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	return p.chatRaw(ctx, body)
}

func (p *OpenAICompatible) chatRaw(ctx context.Context, body map[string]any) (ChatResponse, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return ChatResponse{}, err
	}
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		openAIURL(p.cfg.BaseURL, "/chat/completions"),
		bytes.NewReader(data),
	)
	if err != nil {
		return ChatResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	p.authorize(httpReq)

	res, err := p.client.Do(httpReq)
	if err != nil {
		return ChatResponse{}, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return ChatResponse{}, fmt.Errorf("chat: %s: %s", res.Status, strings.TrimSpace(string(msg)))
	}

	var payload struct {
		Choices []struct {
			Message Message `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return ChatResponse{}, err
	}
	if len(payload.Choices) == 0 {
		return ChatResponse{}, errors.New("chat response has no choices")
	}
	return ChatResponse{Content: payload.Choices[0].Message.Content}, nil
}

func (p *OpenAICompatible) authorize(req *http.Request) {
	if p.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	}
	for key, value := range p.cfg.Headers {
		req.Header.Set(key, value)
	}
}

func openAIURL(baseURL string, path string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	if strings.HasSuffix(baseURL, "/v1") {
		return baseURL + path
	}
	return baseURL + "/v1" + path
}

type Ollama struct {
	cfg    config.ProviderConfig
	client *http.Client
}

func NewOllama(cfg config.ProviderConfig, client *http.Client) *Ollama {
	return &Ollama{cfg: cfg, client: client}
}

func (p *Ollama) ID() string { return p.cfg.ID }

func (p *Ollama) Health(ctx context.Context) error {
	_, err := p.ListModels(ctx)
	return err
}

func (p *Ollama) ListModels(ctx context.Context) ([]Model, error) {
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
	models := make([]Model, 0, len(payload.Models))
	for _, item := range payload.Models {
		if item.Name != "" {
			models = append(models, Model{ID: item.Name, Name: item.Name})
		}
	}
	return models, nil
}

func (p *Ollama) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = p.cfg.Model
	}
	if model == "" {
		return ChatResponse{}, errors.New("model is required")
	}

	body := map[string]any{
		"model":    model,
		"messages": req.Messages,
		"stream":   false,
		"options": map[string]any{
			"temperature": req.Temperature,
		},
	}
	data, err := json.Marshal(body)
	if err != nil {
		return ChatResponse{}, err
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(p.cfg.BaseURL, "/")+"/api/chat",
		bytes.NewReader(data),
	)
	if err != nil {
		return ChatResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	res, err := p.client.Do(httpReq)
	if err != nil {
		return ChatResponse{}, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return ChatResponse{}, fmt.Errorf("chat: %s: %s", res.Status, strings.TrimSpace(string(msg)))
	}

	var payload struct {
		Message Message `json:"message"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return ChatResponse{}, err
	}
	return ChatResponse{Content: payload.Message.Content}, nil
}

func (p *Ollama) ChatStream(ctx context.Context, req ChatRequest, yield func(string) error) error {
	model := req.Model
	if model == "" {
		model = p.cfg.Model
	}
	if model == "" {
		return errors.New("model is required")
	}
	body := map[string]any{
		"model":    model,
		"messages": req.Messages,
		"stream":   true,
		"options": map[string]any{
			"temperature": req.Temperature,
		},
	}
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(p.cfg.BaseURL, "/")+"/api/chat",
		bytes.NewReader(data),
	)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	res, err := p.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return fmt.Errorf("chat stream: %s: %s", res.Status, strings.TrimSpace(string(msg)))
	}
	scanner := bufio.NewScanner(res.Body)
	for scanner.Scan() {
		var chunk struct {
			Message Message `json:"message"`
			Done    bool    `json:"done"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
			continue
		}
		if chunk.Message.Content != "" {
			if err := yield(chunk.Message.Content); err != nil {
				return err
			}
		}
		if chunk.Done {
			return nil
		}
	}
	return scanner.Err()
}

func (p *Ollama) Vision(ctx context.Context, req VisionRequest) (ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = p.cfg.Model
	}
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
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(p.cfg.BaseURL, "/")+"/api/chat",
		bytes.NewReader(data),
	)
	if err != nil {
		return ChatResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

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
