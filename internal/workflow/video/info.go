package video

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

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
