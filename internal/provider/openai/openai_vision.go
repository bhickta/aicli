package openai

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"

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
	images := visionImages(req)
	if len(images) == 0 {
		return provider.ChatResponse{}, errors.New("image is required")
	}
	content := []map[string]any{{"type": "text", "text": req.Prompt}}
	for _, image := range images {
		if name := strings.TrimSpace(image.Name); name != "" {
			content = append(content, map[string]any{"type": "text", "text": "Image name: " + name})
		}
		mimeType := image.MIMEType
		if mimeType == "" {
			mimeType = "image/jpeg"
		}
		content = append(content, map[string]any{
			"type": "image_url",
			"image_url": map[string]any{
				"url": "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(image.Image),
			},
		})
	}

	body := map[string]any{
		"model": model,
		"messages": []map[string]any{
			{
				"role":    "user",
				"content": content,
			},
		},
		"temperature": req.Temperature,
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	return p.chatRaw(ctx, body)
}

func visionImages(req provider.VisionRequest) []provider.VisionImage {
	if len(req.Images) > 0 {
		return req.Images
	}
	if len(req.Image) == 0 {
		return nil
	}
	return []provider.VisionImage{{
		Image:    req.Image,
		MIMEType: req.MIMEType,
	}}
}
