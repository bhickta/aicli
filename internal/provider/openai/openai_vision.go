package openai

import (
	"context"
	"encoding/base64"
	"errors"

	"github.com/bhickta/aicli/internal/provider"
)

func (p *OpenAICompatible) Vision(ctx context.Context, req provider.VisionRequest) (provider.ChatResponse, error) {
	if p.usesResponsesAPI() {
		return provider.ChatResponse{}, errors.New("vision is not supported by this Responses API provider")
	}
	model := p.chatModel(req.Model)
	if model == "" {
		return provider.ChatResponse{}, errors.New("model is required")
	}
	if len(req.Image) == 0 {
		return provider.ChatResponse{}, errors.New("image is required")
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
