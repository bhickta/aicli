package openai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

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
	data, err := json.Marshal(body)
	if err != nil {
		return provider.DocumentResponse{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.geminiGenerateContentURL(model), bytes.NewReader(data))
	if err != nil {
		return provider.DocumentResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	p.authorizeGemini(httpReq)
	res, err := p.client.Do(httpReq)
	if err != nil {
		return provider.DocumentResponse{}, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return provider.DocumentResponse{}, p.apiStatusError("document", res.Status, msg)
	}
	var payload struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return provider.DocumentResponse{}, err
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
	}, nil
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
