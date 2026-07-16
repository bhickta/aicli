package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/bhickta/aicli/internal/provider"
)

func (p *OpenAICompatible) Rerank(ctx context.Context, req provider.RerankRequest) (provider.RerankResponse, error) {
	body := map[string]any{"model": p.chatModel(req.Model), "query": req.Query, "documents": req.Documents}
	data, err := json.Marshal(body)
	if err != nil {
		return provider.RerankResponse{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIURL(p.cfg.BaseURL, "/rerank"), bytes.NewReader(data))
	if err != nil {
		return provider.RerankResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	p.authorize(httpReq)
	res, err := p.client.Do(httpReq)
	if err != nil {
		return provider.RerankResponse{}, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		message, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return provider.RerankResponse{}, fmt.Errorf("rerank: %s: %s", res.Status, strings.TrimSpace(string(message)))
	}
	return decodeRerankResponse(res.Body)
}

func decodeRerankResponse(reader io.Reader) (provider.RerankResponse, error) {
	var payload struct {
		Results []struct {
			Index          int     `json:"index"`
			RelevanceScore float64 `json:"relevance_score"`
			Score          float64 `json:"score"`
		} `json:"results"`
	}
	if err := json.NewDecoder(reader).Decode(&payload); err != nil {
		return provider.RerankResponse{}, err
	}
	response := provider.RerankResponse{Results: make([]provider.RerankResult, 0, len(payload.Results))}
	for _, result := range payload.Results {
		score := result.RelevanceScore
		if score == 0 {
			score = result.Score
		}
		response.Results = append(response.Results, provider.RerankResult{Index: result.Index, Score: score})
	}
	return response, nil
}
