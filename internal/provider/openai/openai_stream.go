package openai

import (
	"bufio"
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

func (p *OpenAICompatible) ChatStream(ctx context.Context, req provider.ChatRequest, yield func(string) error) error {
	model := p.chatModel(req.Model)
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
	return scanOpenAIStream(res.Body, yield)
}

func scanOpenAIStream(body io.Reader, yield func(string) error) error {
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if line == "[DONE]" {
			return nil
		}
		content := openAIStreamContent(line)
		if content == "" {
			continue
		}
		if err := yield(content); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func openAIStreamContent(line string) string {
	var chunk struct {
		Choices []struct {
			Delta struct {
				Content string `json:"content"`
			} `json:"delta"`
		} `json:"choices"`
	}
	if err := json.Unmarshal([]byte(line), &chunk); err != nil {
		return ""
	}
	if len(chunk.Choices) == 0 {
		return ""
	}
	return chunk.Choices[0].Delta.Content
}
