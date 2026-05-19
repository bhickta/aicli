package audio

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/tool"
	"github.com/bhickta/aicli/internal/workflow/whisper"
)

type Service struct {
	tools    config.ToolConfig
	runner   tool.Runner
	provider provider.Provider
}

type TranscribeRequest struct {
	Path  string `json:"path"`
	Model string `json:"model"`
}

type TranscribeResponse struct {
	TextPath string `json:"text_path"`
	Text     string `json:"text"`
}

type AnalyzeRequest struct {
	Model      string   `json:"model"`
	TrackText  []string `json:"track_text"`
	TrackNames []string `json:"track_names"`
}

type AnalyzeResponse struct {
	Analysis  string `json:"analysis"`
	Playlists string `json:"playlists"`
}

func New(tools config.ToolConfig, runner tool.Runner, providers ...provider.Provider) *Service {
	var p provider.Provider
	if len(providers) > 0 {
		p = providers[0]
	}
	return &Service{tools: tools, runner: runner, provider: p}
}

func (s *Service) Transcribe(ctx context.Context, req TranscribeRequest) (TranscribeResponse, error) {
	if strings.TrimSpace(req.Path) == "" {
		return TranscribeResponse{}, errors.New("path is required")
	}
	outBase := strings.TrimSuffix(req.Path, filepath.Ext(req.Path))
	out, err := whisper.Run(ctx, s.runner, whisper.Request{
		Command:    s.tools.WhisperCLI,
		AudioPath:  req.Path,
		OutputBase: outBase,
		Model:      req.Model,
		Text:       true,
	})
	if err != nil {
		return TranscribeResponse{}, whisper.OutputError(out, err)
	}
	textPath := outBase + ".txt"
	text, readErr := os.ReadFile(textPath)
	if readErr != nil {
		return TranscribeResponse{TextPath: textPath, Text: strings.TrimSpace(string(out))}, nil
	}
	return TranscribeResponse{TextPath: textPath, Text: strings.TrimSpace(string(text))}, nil
}

func (s *Service) Analyze(ctx context.Context, req AnalyzeRequest) (AnalyzeResponse, error) {
	if s.provider == nil {
		return AnalyzeResponse{}, errors.New("provider is required")
	}
	if len(req.TrackText) == 0 {
		return AnalyzeResponse{}, errors.New("track_text is required")
	}
	payload, _ := json.Marshal(req)
	analysis, err := s.provider.Chat(ctx, provider.ChatRequest{
		Model: req.Model,
		Messages: []provider.Message{
			{Role: "user", Content: "Analyze these audio transcripts. For each track, identify topic, quality, mood, usefulness, and tags. Output concise Markdown.\n\n" + string(payload)},
		},
		Temperature: 0.1,
		MaxTokens:   3000,
	})
	if err != nil {
		return AnalyzeResponse{}, err
	}
	playlists, err := s.provider.Chat(ctx, provider.ChatRequest{
		Model: req.Model,
		Messages: []provider.Message{
			{Role: "user", Content: "Group these audio tracks into useful playlists. Return playlist names and included track indexes as Markdown.\n\n" + string(payload)},
		},
		Temperature: 0.2,
		MaxTokens:   3000,
	})
	if err != nil {
		return AnalyzeResponse{}, err
	}
	return AnalyzeResponse{Analysis: strings.TrimSpace(analysis.Content), Playlists: strings.TrimSpace(playlists.Content)}, nil
}
