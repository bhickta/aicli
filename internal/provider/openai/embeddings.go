package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/bhickta/aicli/internal/provider"
)

func (p *OpenAICompatible) Embeddings(ctx context.Context, req provider.EmbeddingRequest) (provider.EmbeddingResponse, error) {
	model := p.chatModel(req.Model)
	if model == "" {
		return provider.EmbeddingResponse{}, errors.New("embedding model is required")
	}
	if len(req.Inputs) == 0 {
		return provider.EmbeddingResponse{Vectors: [][]float64{}}, nil
	}
	input := any(req.Inputs)
	if len(req.Inputs) == 1 {
		input = req.Inputs[0]
	}
	body := map[string]any{
		"model": model,
		"input": input,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return provider.EmbeddingResponse{}, err
	}
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		openAIURL(p.cfg.BaseURL, "/embeddings"),
		bytes.NewReader(data),
	)
	if err != nil {
		return provider.EmbeddingResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	p.authorize(httpReq)

	res, err := p.client.Do(httpReq)
	if err != nil {
		return provider.EmbeddingResponse{}, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return provider.EmbeddingResponse{}, fmt.Errorf("embeddings: %s: %s", res.Status, strings.TrimSpace(string(msg)))
	}

	var payload struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return provider.EmbeddingResponse{}, err
	}
	if len(payload.Data) != len(req.Inputs) {
		return provider.EmbeddingResponse{}, fmt.Errorf("embedding response returned %d item(s) for %d input(s)", len(payload.Data), len(req.Inputs))
	}
	sort.SliceStable(payload.Data, func(i, j int) bool {
		return payload.Data[i].Index < payload.Data[j].Index
	})
	vectors := make([][]float64, 0, len(payload.Data))
	for _, row := range payload.Data {
		if len(row.Embedding) == 0 {
			return provider.EmbeddingResponse{}, errors.New("embedding response included an empty vector")
		}
		vectors = append(vectors, row.Embedding)
	}
	return provider.EmbeddingResponse{Vectors: vectors}, nil
}
