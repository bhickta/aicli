package info

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/tool"
)

type Request struct {
	Path string `json:"path"`
}

type Response struct {
	Raw     json.RawMessage `json:"raw"`
	Summary string          `json:"summary"`
}

type Service struct {
	tools  config.ToolConfig
	runner tool.Runner
}

func New(tools config.ToolConfig, runner tool.Runner) *Service {
	return &Service{tools: tools, runner: runner}
}

func (s *Service) Run(ctx context.Context, req Request) (Response, error) {
	if strings.TrimSpace(req.Path) == "" {
		return Response{}, errors.New("path is required")
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
		return Response{}, errors.New(strings.TrimSpace(string(out)) + ": " + err.Error())
	}
	if !json.Valid(out) {
		return Response{}, errors.New("ffprobe did not return valid JSON")
	}
	return Response{Raw: out, Summary: summarize(out)}, nil
}

func summarize(raw []byte) string {
	var payload struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			CodecName string `json:"codec_name"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
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
