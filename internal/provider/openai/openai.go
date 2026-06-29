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

func (p *OpenAICompatible) LocalModelServer() bool {
	if strings.EqualFold(strings.TrimSpace(p.cfg.Type), "vllm") {
		return true
	}
	baseURL := strings.ToLower(p.cfg.BaseURL)
	return p.cfg.ID == "lms" ||
		strings.Contains(baseURL, "localhost:1234") ||
		strings.Contains(baseURL, "127.0.0.1:1234")
}

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
		return nil, p.apiStatusError("list models "+modelsURL, res.Status, msg)
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
		"messages":    normalizedMessages(req.Messages),
		"temperature": req.Temperature,
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	applyPromptCacheOptions(body, req, p.cfg, false, model)
	return p.chatRaw(ctx, body)
}

func (p *OpenAICompatible) UnloadModel(ctx context.Context, model string) error {
	if p.cfg.ID != "lms" && !strings.Contains(strings.ToLower(p.cfg.BaseURL), "localhost:1234") && !strings.Contains(strings.ToLower(p.cfg.BaseURL), "127.0.0.1:1234") {
		return nil
	}
	model = strings.TrimSpace(model)
	if model == "" {
		model = p.cfg.Model
	}
	if model == "" {
		return nil
	}
	body := map[string]any{
		"instance_id": model,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	baseURL := strings.TrimRight(p.cfg.BaseURL, "/")
	if strings.HasSuffix(baseURL, "/v1") {
		baseURL = strings.TrimSuffix(baseURL, "/v1")
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/v1/models/unload", bytes.NewReader(data))
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
	if res.StatusCode == http.StatusNotFound {
		return nil
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return p.apiStatusError("unload model", res.Status, msg)
	}
	return nil
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
		return provider.ChatResponse{}, p.apiStatusError("chat", res.Status, msg)
	}

	var payload struct {
		Choices []struct {
			Message      provider.Message `json:"message"`
			FinishReason string           `json:"finish_reason"`
		} `json:"choices"`
		Usage chatCompletionUsage `json:"usage"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return provider.ChatResponse{}, err
	}
	if len(payload.Choices) == 0 {
		return provider.ChatResponse{}, errors.New("chat response has no choices")
	}
	return provider.ChatResponse{
		Content:      payload.Choices[0].Message.Content,
		FinishReason: payload.Choices[0].FinishReason,
		Usage:        payload.Usage.providerUsage(),
	}, nil
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

func (p *OpenAICompatible) apiStatusError(operation string, status string, body []byte) error {
	if p.resolvedAPIKey() == "" && strings.HasPrefix(status, "401") {
		return fmt.Errorf("%s: %s: missing api authentication for provider %q; %s", operation, status, p.cfg.ID, p.authenticationHint())
	}
	message := strings.TrimSpace(string(body))
	if message == "" {
		return fmt.Errorf("%s: %s", operation, status)
	}
	return fmt.Errorf("%s: %s: %s", operation, status, message)
}

func (p *OpenAICompatible) authenticationHint() string {
	hint := "set the provider api_key in Settings"
	if p.cfg.APIKeyEnv != "" {
		hint = "set " + p.cfg.APIKeyEnv + " before starting aicli, or set the provider api_key in Settings"
	}
	if p.cfg.ID == "codex" || p.usesResponsesAPI() {
		hint += "; for Codex Pro plan usage choose Workflows > Codex > Coding task (Codex CLI / Pro)"
	}
	return hint
}

func openAIURL(baseURL string, path string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	if strings.HasSuffix(baseURL, "/v1") || strings.HasSuffix(baseURL, "/openai") {
		return baseURL + path
	}
	return baseURL + "/v1" + path
}

func normalizedMessages(messages []provider.Message) []provider.Message {
	out := make([]provider.Message, 0, len(messages))
	for _, message := range messages {
		if strings.TrimSpace(message.Content) == "" {
			continue
		}
		role := strings.TrimSpace(message.Role)
		if role == "" {
			role = "user"
		}
		out = append(out, provider.Message{Role: role, Content: message.Content})
	}
	return out
}

func applyPromptCacheOptions(body map[string]any, req provider.ChatRequest, cfg config.ProviderConfig, useDefault bool, model string) {
	cacheKey := firstNonBlank(req.PromptCacheKey, cfg.PromptCacheKey)
	if cacheKey == "" && useDefault {
		cacheKey = defaultPromptCacheKey(cfg.ID, model)
	}
	if cacheKey != "" {
		body["prompt_cache_key"] = cacheKey
	}
	if retention := firstNonBlank(req.PromptCacheRetention, cfg.PromptCacheRetention); retention != "" {
		body["prompt_cache_retention"] = retention
	}
}

func defaultPromptCacheKey(providerID string, model string) string {
	providerID = cacheKeyPart(providerID)
	model = cacheKeyPart(model)
	if providerID == "" && model == "" {
		return ""
	}
	if providerID == "" {
		return "aicli-" + model
	}
	if model == "" {
		return "aicli-" + providerID
	}
	return "aicli-" + providerID + "-" + model
}

func cacheKeyPart(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	var out strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			out.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			out.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(out.String(), "-")
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

type chatCompletionUsage struct {
	PromptTokens            int                      `json:"prompt_tokens"`
	CompletionTokens        int                      `json:"completion_tokens"`
	TotalTokens             int                      `json:"total_tokens"`
	PromptTokensDetails     chatCompletionDetails    `json:"prompt_tokens_details"`
	CompletionTokensDetails chatCompletionOutDetails `json:"completion_tokens_details"`
}

func (u chatCompletionUsage) providerUsage() *provider.TokenUsage {
	if u.PromptTokens == 0 && u.CompletionTokens == 0 && u.TotalTokens == 0 &&
		u.PromptTokensDetails.CachedTokens == 0 && u.CompletionTokensDetails.ReasoningTokens == 0 {
		return nil
	}
	return &provider.TokenUsage{
		InputTokens:           u.PromptTokens,
		CachedInputTokens:     u.PromptTokensDetails.CachedTokens,
		OutputTokens:          u.CompletionTokens,
		ReasoningOutputTokens: u.CompletionTokensDetails.ReasoningTokens,
		TotalTokens:           u.TotalTokens,
	}
}

type chatCompletionDetails struct {
	CachedTokens int `json:"cached_tokens"`
}

type chatCompletionOutDetails struct {
	ReasoningTokens int `json:"reasoning_tokens"`
}
