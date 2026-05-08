package video

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/tool"
)

type Service struct {
	tools    config.ToolConfig
	runner   tool.Runner
	provider provider.Provider
}

type InfoRequest struct {
	Path string `json:"path"`
}

type InfoResponse struct {
	Raw     json.RawMessage `json:"raw"`
	Summary string          `json:"summary"`
}

type CompressRequest struct {
	Path   string `json:"path"`
	Output string `json:"output"`
	CRF    int    `json:"crf"`
}

type CompressResponse struct {
	Output string `json:"output"`
}

type MetadataRequest struct {
	Path    string `json:"path"`
	Sidecar string `json:"sidecar"`
	Output  string `json:"output"`
}

type MetadataResponse struct {
	Sidecar string `json:"sidecar,omitempty"`
	Output  string `json:"output,omitempty"`
}

type LLMRequest struct {
	Model      string `json:"model"`
	Title      string `json:"title"`
	Transcript string `json:"transcript"`
	Mode       string `json:"mode"`
}

type LLMResponse struct {
	Text string `json:"text"`
}

func New(tools config.ToolConfig, runner tool.Runner, providers ...provider.Provider) *Service {
	var p provider.Provider
	if len(providers) > 0 {
		p = providers[0]
	}
	return &Service{tools: tools, runner: runner, provider: p}
}

func (s *Service) Info(ctx context.Context, req InfoRequest) (InfoResponse, error) {
	if strings.TrimSpace(req.Path) == "" {
		return InfoResponse{}, errors.New("path is required")
	}
	out, err := s.runner.CombinedOutput(
		ctx,
		s.tools.FFprobe,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		req.Path,
	)
	if err != nil {
		return InfoResponse{}, errors.New(strings.TrimSpace(string(out)) + ": " + err.Error())
	}
	if !json.Valid(out) {
		return InfoResponse{}, errors.New("ffprobe did not return valid JSON")
	}
	return InfoResponse{Raw: out, Summary: summarize(out)}, nil
}

func (s *Service) Compress(ctx context.Context, req CompressRequest) (CompressResponse, error) {
	if strings.TrimSpace(req.Path) == "" {
		return CompressResponse{}, errors.New("path is required")
	}
	output := req.Output
	if output == "" {
		output = strings.TrimSuffix(req.Path, filepathExt(req.Path)) + ".compressed.mp4"
	}
	crf := req.CRF
	if crf == 0 {
		crf = 28
	}
	out, err := s.runner.CombinedOutput(
		ctx,
		s.tools.FFmpeg,
		"-y",
		"-i", req.Path,
		"-c:v", "libx264",
		"-crf", itoa(crf),
		"-preset", "medium",
		"-c:a", "aac",
		"-b:a", "128k",
		output,
	)
	if err != nil {
		return CompressResponse{}, errors.New(strings.TrimSpace(string(out)) + ": " + err.Error())
	}
	return CompressResponse{Output: output}, nil
}

func (s *Service) BackupMetadata(ctx context.Context, req MetadataRequest) (MetadataResponse, error) {
	if strings.TrimSpace(req.Path) == "" {
		return MetadataResponse{}, errors.New("path is required")
	}
	sidecar := req.Sidecar
	if sidecar == "" {
		sidecar = req.Path + ".ffmetadata"
	}
	out, err := s.runner.CombinedOutput(
		ctx,
		s.tools.FFmpeg,
		"-y",
		"-i", req.Path,
		"-f", "ffmetadata",
		sidecar,
	)
	if err != nil {
		return MetadataResponse{}, errors.New(strings.TrimSpace(string(out)) + ": " + err.Error())
	}
	return MetadataResponse{Sidecar: sidecar}, nil
}

func (s *Service) RestoreMetadata(ctx context.Context, req MetadataRequest) (MetadataResponse, error) {
	if strings.TrimSpace(req.Path) == "" || strings.TrimSpace(req.Sidecar) == "" {
		return MetadataResponse{}, errors.New("path and sidecar are required")
	}
	output := req.Output
	if output == "" {
		output = strings.TrimSuffix(req.Path, filepathExt(req.Path)) + ".restored" + filepathExt(req.Path)
	}
	out, err := s.runner.CombinedOutput(
		ctx,
		s.tools.FFmpeg,
		"-y",
		"-i", req.Path,
		"-i", req.Sidecar,
		"-map_metadata", "1",
		"-codec", "copy",
		output,
	)
	if err != nil {
		return MetadataResponse{}, errors.New(strings.TrimSpace(string(out)) + ": " + err.Error())
	}
	return MetadataResponse{Sidecar: req.Sidecar, Output: output}, nil
}

func (s *Service) Generate(ctx context.Context, req LLMRequest) (LLMResponse, error) {
	if s.provider == nil {
		return LLMResponse{}, errors.New("provider is required")
	}
	if strings.TrimSpace(req.Transcript) == "" {
		return LLMResponse{}, errors.New("transcript is required")
	}
	prompt, err := videoPrompt(req)
	if err != nil {
		return LLMResponse{}, err
	}
	res, err := s.provider.Chat(ctx, provider.ChatRequest{
		Model: req.Model,
		Messages: []provider.Message{
			{Role: "user", Content: prompt},
		},
		Temperature: 0.1,
		MaxTokens:   3500,
	})
	if err != nil {
		return LLMResponse{}, err
	}
	return LLMResponse{Text: strings.TrimSpace(res.Content)}, nil
}

func summarize(raw []byte) string {
	var payload struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			CodecName string `json:"codec_name"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
			Size     string `json:"size"`
		} `json:"format"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	parts := []string{}
	for _, stream := range payload.Streams {
		if stream.CodecType == "video" {
			parts = append(parts, "video="+stream.CodecName)
		}
		if stream.CodecType == "audio" {
			parts = append(parts, "audio="+stream.CodecName)
		}
	}
	if payload.Format.Duration != "" {
		parts = append(parts, "duration="+payload.Format.Duration)
	}
	return strings.Join(parts, " ")
}

func videoPrompt(req LLMRequest) (string, error) {
	title := req.Title
	if title == "" {
		title = "Untitled video"
	}
	switch req.Mode {
	case "", "notes":
		return "Create high-signal study notes for this video transcript. Include headings, bullets, key terms, and action items.\nTitle: " + title + "\n\n" + req.Transcript, nil
	case "tags":
		return "Generate concise searchable tags for this video. Output JSON with keys title, summary, tags, difficulty, topics.\nTitle: " + title + "\n\n" + req.Transcript, nil
	case "course":
		return "Turn this video transcript into a course module plan. Include module title, learning objectives, lesson outline, quiz questions, and prerequisites.\nTitle: " + title + "\n\n" + req.Transcript, nil
	default:
		return "", errors.New("unsupported video LLM mode")
	}
}

func filepathExt(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			return path[i:]
		}
		if path[i] == '/' {
			break
		}
	}
	return ""
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := []byte{}
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}
