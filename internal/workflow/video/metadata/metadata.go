package metadata

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/tool"
)

type Request struct {
	Path    string `json:"path"`
	Sidecar string `json:"sidecar"`
	Output  string `json:"output"`
}

type Response struct {
	Sidecar string `json:"sidecar,omitempty"`
	Output  string `json:"output,omitempty"`
}

type Service struct {
	tools  config.ToolConfig
	runner tool.Runner
}

func New(tools config.ToolConfig, runner tool.Runner) *Service {
	return &Service{tools: tools, runner: runner}
}

func (s *Service) Backup(ctx context.Context, req Request) (Response, error) {
	if strings.TrimSpace(req.Path) == "" {
		return Response{}, errors.New("path is required")
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
		return Response{}, errors.New(strings.TrimSpace(string(out)) + ": " + err.Error())
	}
	return Response{Sidecar: sidecar}, nil
}

func (s *Service) Restore(ctx context.Context, req Request) (Response, error) {
	if strings.TrimSpace(req.Path) == "" || strings.TrimSpace(req.Sidecar) == "" {
		return Response{}, errors.New("path and sidecar are required")
	}
	output := req.Output
	if output == "" {
		ext := filepath.Ext(req.Path)
		output = strings.TrimSuffix(req.Path, ext) + ".restored" + ext
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
		return Response{}, errors.New(strings.TrimSpace(string(out)) + ": " + err.Error())
	}
	return Response{Sidecar: req.Sidecar, Output: output}, nil
}
