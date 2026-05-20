package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/bhickta/aicli/internal/provider"
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
		return provider.ChatResponse{}, fmt.Errorf("responses chat: %s: %s", res.Status, strings.TrimSpace(string(msg)))
	}

	content, err := decodeResponseText(res.Body)
	if err != nil {
		return provider.ChatResponse{}, err
	}
	return provider.ChatResponse{Content: content}, nil
}

func (p *OpenAICompatible) responsesBody(model string, req provider.ChatRequest) map[string]any {
	body := map[string]any{
		"model": model,
		"input": req.Messages,
		"store": false,
	}
	if req.MaxTokens > 0 {
		body["max_output_tokens"] = req.MaxTokens
	}
	if effort := firstNonEmpty(req.ReasoningEffort, p.cfg.ReasoningEffort); effort != "" {
		body["reasoning"] = map[string]any{"effort": effort}
	}
	if verbosity := firstNonEmpty(req.TextVerbosity, p.cfg.TextVerbosity); verbosity != "" {
		body["text"] = map[string]any{"verbosity": verbosity}
	}
	return body
}

func decodeResponseText(body io.Reader) (string, error) {
	var payload struct {
		OutputText string `json:"output_text"`
		Output     []struct {
			Type    string `json:"type"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.NewDecoder(body).Decode(&payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.OutputText) != "" {
		return payload.OutputText, nil
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
		return "", errors.New("response has no output text")
	}
	return text.String(), nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
