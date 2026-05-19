package video

import (
	"context"
	"errors"
	"strings"
)

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
