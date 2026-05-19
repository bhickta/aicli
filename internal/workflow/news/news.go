package news

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/bhickta/aicli/internal/provider"
)

type Item struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Source  string `json:"source"`
	Date    string `json:"date"`
}

type Request struct {
	Model      string `json:"model"`
	Path       string `json:"path"`
	OutputPath string `json:"output_path"`
	UseLLM     bool   `json:"use_llm"`
	ProviderID string `json:"provider_id"`
}

type Response struct {
	Items       []Item    `json:"items"`
	Clusters    []Cluster `json:"clusters"`
	Duplicates  int       `json:"duplicates"`
	LLMSummary  string    `json:"llm_summary,omitempty"`
	SourceCount int       `json:"source_count"`
	OutputPath  string    `json:"output_path,omitempty"`
}

type Cluster struct {
	Items []Item  `json:"items"`
	Score float64 `json:"score"`
}

type Service struct {
	provider provider.Provider
}

func New(provider provider.Provider) *Service {
	return &Service{provider: provider}
}

func (s *Service) Run(ctx context.Context, req Request) (Response, error) {
	if strings.TrimSpace(req.Path) == "" {
		return Response{}, errors.New("path is required")
	}
	items, err := loadItems(req.Path)
	if err != nil {
		return Response{}, err
	}
	deduped, duplicates := dedupe(items)
	res := Response{Items: deduped, Clusters: cluster(deduped, 0.35), Duplicates: duplicates, SourceCount: len(items)}
	if req.OutputPath != "" {
		if err := exportXLSX(req.OutputPath, deduped); err != nil {
			return Response{}, err
		}
		res.OutputPath = req.OutputPath
	}
	if req.UseLLM && s.provider != nil && len(deduped) > 0 {
		summary, err := s.summarize(ctx, req.Model, deduped)
		if err != nil {
			return Response{}, err
		}
		res.LLMSummary = summary
	}
	return res, nil
}

func (s *Service) summarize(ctx context.Context, model string, items []Item) (string, error) {
	payload, _ := json.Marshal(items)
	chat, err := s.provider.Chat(ctx, provider.ChatRequest{
		Model: model,
		Messages: []provider.Message{
			{Role: "user", Content: "Merge and summarize these deduplicated news items. Output concise Markdown.\n\n" + string(payload)},
		},
		Temperature: 0,
		MaxTokens:   2000,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(chat.Content), nil
}
