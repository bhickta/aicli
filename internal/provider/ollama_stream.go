package provider

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

func (p *Ollama) ChatStream(ctx context.Context, req ChatRequest, yield func(string) error) error {
	model := p.chatModel(req.Model)
	if model == "" {
		return errors.New("model is required")
	}
	body := ollamaChatBody(model, req, true)
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	httpReq, err := p.chatRequest(ctx, data)
	if err != nil {
		return err
	}
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
