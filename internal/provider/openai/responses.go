package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/textutil"
)

const responsesProviderType = "openai-responses"

func (p *OpenAICompatible) usesResponsesAPI() bool {
	return p.cfg.Type == responsesProviderType
}

func (p *OpenAICompatible) responsesChat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	model := p.chatModel(req.Model)
	if model == "" {
		return provider.ChatResponse{}, errors.New("model is required")
	}
	body := p.responsesBody(model, req)
	data, err := json.Marshal(body)
	if err != nil {
		return provider.ChatResponse{}, err
	}
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		openAIURL(p.cfg.BaseURL, "/responses"),
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
		return provider.ChatResponse{}, p.apiStatusError("responses chat", res.Status, msg)
	}

	chat, err := decodeResponse(res.Body)
	if err != nil {
		return provider.ChatResponse{}, err
	}
	return chat, nil
}

func (p *OpenAICompatible) responsesBody(model string, req provider.ChatRequest) map[string]any {
	body := map[string]any{
		"model": model,
		"input": normalizedMessages(req.Messages),
		"store": false,
	}
	if req.MaxTokens > 0 {
		body["max_output_tokens"] = req.MaxTokens
	}
	if effort := textutil.FirstNonBlank(req.ReasoningEffort, p.cfg.ReasoningEffort); effort != "" {
		body["reasoning"] = map[string]any{"effort": effort}
	}
	if verbosity := textutil.FirstNonBlank(req.TextVerbosity, p.cfg.TextVerbosity); verbosity != "" {
		body["text"] = map[string]any{"verbosity": verbosity}
	}
	if cacheKey := textutil.FirstNonBlank(req.PromptCacheKey, p.cfg.PromptCacheKey, defaultPromptCacheKey(p.cfg.ID, model)); cacheKey != "" {
		body["prompt_cache_key"] = cacheKey
	}
	if retention := textutil.FirstNonBlank(req.PromptCacheRetention, p.cfg.PromptCacheRetention); retention != "" {
		body["prompt_cache_retention"] = retention
	}
	return body
}

func decodeResponse(body io.Reader) (provider.ChatResponse, error) {
	var payload struct {
		OutputText string `json:"output_text"`
		Output     []struct {
			Type    string `json:"type"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
		Usage responseUsage `json:"usage"`
	}
	if err := json.NewDecoder(body).Decode(&payload); err != nil {
		return provider.ChatResponse{}, err
	}
	if strings.TrimSpace(payload.OutputText) != "" {
		return provider.ChatResponse{Content: payload.OutputText, Usage: payload.Usage.providerUsage()}, nil
	}

	var text strings.Builder
	for _, item := range payload.Output {
		if item.Type != "" && item.Type != "message" {
			continue
		}
		for _, content := range item.Content {
			if content.Text == "" {
				continue
			}
			if text.Len() > 0 {
				text.WriteString("\n")
			}
			text.WriteString(content.Text)
		}
	}
	if text.Len() == 0 {
		return provider.ChatResponse{}, errors.New("response has no output text")
	}
	return provider.ChatResponse{Content: text.String(), Usage: payload.Usage.providerUsage()}, nil
}

type responseUsage struct {
	InputTokens        int          `json:"input_tokens"`
	OutputTokens       int          `json:"output_tokens"`
	TotalTokens        int          `json:"total_tokens"`
	InputTokenDetails  tokenDetails `json:"input_tokens_details"`
	OutputTokenDetails tokenDetails `json:"output_tokens_details"`
}

func (u responseUsage) providerUsage() *provider.TokenUsage {
	if u.InputTokens == 0 && u.OutputTokens == 0 && u.TotalTokens == 0 &&
		u.InputTokenDetails.CachedTokens == 0 && u.OutputTokenDetails.ReasoningTokens == 0 {
		return nil
	}
	return &provider.TokenUsage{
		InputTokens:           u.InputTokens,
		CachedInputTokens:     u.InputTokenDetails.CachedTokens,
		OutputTokens:          u.OutputTokens,
		ReasoningOutputTokens: u.OutputTokenDetails.ReasoningTokens,
		TotalTokens:           u.TotalTokens,
	}
}

type tokenDetails struct {
	CachedTokens    int `json:"cached_tokens"`
	ReasoningTokens int `json:"reasoning_tokens"`
}
