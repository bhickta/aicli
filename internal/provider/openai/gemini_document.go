package openai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bhickta/aicli/internal/provider"
)

func (p *OpenAICompatible) Document(ctx context.Context, req provider.DocumentRequest) (provider.DocumentResponse, error) {
	if !p.usesGeminiGenerateContent() {
		return provider.DocumentResponse{}, errors.New("direct PDF input is only supported for Gemini API providers")
	}
	model := p.chatModel(req.Model)
	if model == "" {
		return provider.DocumentResponse{}, errors.New("model is required")
	}
	if len(req.Data) == 0 {
		return provider.DocumentResponse{}, errors.New("document is required")
	}
	mimeType := strings.TrimSpace(req.MIMEType)
	if mimeType == "" {
		mimeType = "application/pdf"
	}
	body := map[string]any{
		"contents": []map[string]any{
			{
				"parts": []map[string]any{
					{"text": req.Prompt},
					{
						"inline_data": map[string]any{
							"mime_type": mimeType,
							"data":      base64.StdEncoding.EncodeToString(req.Data),
						},
					},
				},
			},
		},
		"generationConfig": map[string]any{
			"temperature": req.Temperature,
		},
	}
	if req.MaxTokens > 0 {
		body["generationConfig"].(map[string]any)["maxOutputTokens"] = req.MaxTokens
	}
	if responseMIMEType := strings.TrimSpace(req.ResponseMIMEType); responseMIMEType != "" {
		body["generationConfig"].(map[string]any)["responseMimeType"] = responseMIMEType
	}
	if req.ResponseSchema != nil {
		body["generationConfig"].(map[string]any)["responseSchema"] = geminiDocumentResponseSchema(req.ResponseSchema.Schema)
	}
	data, err := json.Marshal(body)
	if err != nil {
		return provider.DocumentResponse{}, err
	}
	for attempt := 0; attempt < geminiDocumentMaxAttempts; attempt++ {
		res, err := p.doGeminiDocumentRequest(ctx, model, data)
		if err != nil {
			return provider.DocumentResponse{}, fmt.Errorf("performing gemini document request: %w", err)
		}
		if res.StatusCode >= 200 && res.StatusCode <= 299 {
			return decodeGeminiDocumentResponse(res)
		}
		msg, readErr := io.ReadAll(io.LimitReader(res.Body, 4096))
		closeErr := res.Body.Close()
		if readErr != nil {
			return provider.DocumentResponse{}, fmt.Errorf("reading Gemini document error response: %w", readErr)
		}
		if closeErr != nil {
			return provider.DocumentResponse{}, fmt.Errorf("closing Gemini document error response: %w", closeErr)
		}
		statusErr := p.apiStatusError("document", res.Status, msg)
		delay, retry := geminiDocumentRetryDelay(res.StatusCode, res.Header, msg, attempt, time.Now())
		if !retry || attempt+1 == geminiDocumentMaxAttempts {
			return provider.DocumentResponse{}, statusErr
		}
		if err := waitForGeminiDocumentRetry(ctx, delay); err != nil {
			return provider.DocumentResponse{}, fmt.Errorf("waiting to retry Gemini document request: %w", err)
		}
	}
	return provider.DocumentResponse{}, errors.New("gemini document retry attempts exhausted")
}

const (
	geminiDocumentMaxAttempts   = 3
	geminiDocumentMaxRetryDelay = 2 * time.Minute
)

func (p *OpenAICompatible) doGeminiDocumentRequest(ctx context.Context, model string, data []byte) (*http.Response, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.geminiGenerateContentURL(model), bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	p.authorizeGemini(httpReq)
	return p.client.Do(httpReq)
}

func decodeGeminiDocumentResponse(res *http.Response) (provider.DocumentResponse, error) {
	var payload struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
		UsageMetadata geminiUsageMetadata `json:"usageMetadata"`
	}
	decodeErr := json.NewDecoder(res.Body).Decode(&payload)
	closeErr := res.Body.Close()
	if decodeErr != nil {
		return provider.DocumentResponse{}, fmt.Errorf("decoding gemini document response: %w", decodeErr)
	}
	if closeErr != nil {
		return provider.DocumentResponse{}, fmt.Errorf("closing gemini document response: %w", closeErr)
	}
	if len(payload.Candidates) == 0 {
		return provider.DocumentResponse{}, errors.New("document response has no candidates")
	}
	var out strings.Builder
	for _, part := range payload.Candidates[0].Content.Parts {
		out.WriteString(part.Text)
	}
	return provider.DocumentResponse{
		Content:      out.String(),
		FinishReason: payload.Candidates[0].FinishReason,
		Usage:        payload.UsageMetadata.providerUsage(),
	}, nil
}

func geminiDocumentRetryDelay(statusCode int, header http.Header, body []byte, attempt int, now time.Time) (time.Duration, bool) {
	switch statusCode {
	case http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
	default:
		return 0, false
	}
	if delay, ok := retryAfterDelay(header.Get("Retry-After"), now); ok {
		return boundedGeminiDocumentRetryDelay(delay), true
	}
	if delay, ok := geminiRetryInfoDelay(body); ok {
		return boundedGeminiDocumentRetryDelay(delay), true
	}
	if statusCode == http.StatusTooManyRequests {
		return time.Minute, true
	}
	return boundedGeminiDocumentRetryDelay(time.Second << attempt), true
}

func retryAfterDelay(value string, now time.Time) (time.Duration, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second, true
	}
	when, err := http.ParseTime(value)
	if err != nil {
		return 0, false
	}
	if delay := when.Sub(now); delay > 0 {
		return delay, true
	}
	return 0, true
}

func geminiRetryInfoDelay(body []byte) (time.Duration, bool) {
	var payload struct {
		Error struct {
			Details []struct {
				RetryDelay string `json:"retryDelay"`
			} `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return 0, false
	}
	for _, detail := range payload.Error.Details {
		delay, err := time.ParseDuration(strings.TrimSpace(detail.RetryDelay))
		if err == nil && delay >= 0 {
			return delay, true
		}
	}
	return 0, false
}

func boundedGeminiDocumentRetryDelay(delay time.Duration) time.Duration {
	if delay > geminiDocumentMaxRetryDelay {
		return geminiDocumentMaxRetryDelay
	}
	return delay
}

func waitForGeminiDocumentRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func geminiDocumentResponseSchema(schema map[string]any) map[string]any {
	converted, _ := geminiDocumentSchemaValue(schema).(map[string]any)
	return converted
}

func geminiDocumentSchemaValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		converted := make(map[string]any, len(typed))
		for key, child := range typed {
			// Gemini's GenerateContent responseSchema uses its OpenAPI Schema
			// type, which does not expose JSON Schema's additionalProperties.
			if key == "additionalProperties" {
				continue
			}
			if key == "type" {
				if schemaType, ok := child.(string); ok {
					converted[key] = strings.ToUpper(schemaType)
					continue
				}
			}
			converted[key] = geminiDocumentSchemaValue(child)
		}
		return converted
	case []any:
		converted := make([]any, len(typed))
		for index, child := range typed {
			converted[index] = geminiDocumentSchemaValue(child)
		}
		return converted
	default:
		return value
	}
}

type geminiUsageMetadata struct {
	PromptTokenCount        int `json:"promptTokenCount"`
	CachedContentTokenCount int `json:"cachedContentTokenCount"`
	CandidatesTokenCount    int `json:"candidatesTokenCount"`
	ThoughtsTokenCount      int `json:"thoughtsTokenCount"`
	TotalTokenCount         int `json:"totalTokenCount"`
}

func (u geminiUsageMetadata) providerUsage() *provider.TokenUsage {
	if u.PromptTokenCount == 0 && u.CandidatesTokenCount == 0 && u.ThoughtsTokenCount == 0 && u.TotalTokenCount == 0 && u.CachedContentTokenCount == 0 {
		return nil
	}
	return &provider.TokenUsage{
		InputTokens:           u.PromptTokenCount,
		CachedInputTokens:     u.CachedContentTokenCount,
		OutputTokens:          u.CandidatesTokenCount,
		ReasoningOutputTokens: u.ThoughtsTokenCount,
		TotalTokens:           u.TotalTokenCount,
	}
}

func (p *OpenAICompatible) usesGeminiGenerateContent() bool {
	baseURL := strings.ToLower(strings.TrimSpace(p.cfg.BaseURL))
	return strings.EqualFold(strings.TrimSpace(p.cfg.ID), "gemini") ||
		strings.Contains(baseURL, "generativelanguage.googleapis.com")
}

func (p *OpenAICompatible) geminiGenerateContentURL(model string) string {
	baseURL := strings.TrimRight(p.cfg.BaseURL, "/")
	baseURL = strings.TrimSuffix(baseURL, "/openai")
	escapedModel := url.PathEscape(strings.TrimPrefix(model, "models/"))
	return baseURL + "/models/" + escapedModel + ":generateContent"
}

func (p *OpenAICompatible) authorizeGemini(req *http.Request) {
	if apiKey := p.resolvedAPIKey(); apiKey != "" {
		req.Header.Set("x-goog-api-key", apiKey)
		return
	}
	p.authorize(req)
}

var _ provider.DocumentProcessor = (*OpenAICompatible)(nil)
