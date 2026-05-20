package ollama

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

func (p *Ollama) Embeddings(ctx context.Context, req provider.EmbeddingRequest) (provider.EmbeddingResponse, error) {
	model := p.chatModel(req.Model)
	if model == "" {
		return provider.EmbeddingResponse{}, errors.New("embedding model is required")
	}
	if len(req.Inputs) == 0 {
		return provider.EmbeddingResponse{Vectors: [][]float64{}}, nil
	}
	body := map[string]any{
		"model": model,
		"input": req.Inputs,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return provider.EmbeddingResponse{}, err
	}
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(p.cfg.BaseURL, "/")+"/api/embed",
		bytes.NewReader(data),
	)
	if err != nil {
		return provider.EmbeddingResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

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
		Embeddings [][]float64 `json:"embeddings"`
		Embedding  []float64   `json:"embedding"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return provider.EmbeddingResponse{}, err
	}
	vectors := payload.Embeddings
	if len(vectors) == 0 && len(payload.Embedding) > 0 {
		vectors = [][]float64{payload.Embedding}
	}
	if len(vectors) != len(req.Inputs) {
		return provider.EmbeddingResponse{}, fmt.Errorf("embedding response returned %d item(s) for %d input(s)", len(vectors), len(req.Inputs))
	}
	return provider.EmbeddingResponse{Vectors: vectors}, nil
}
