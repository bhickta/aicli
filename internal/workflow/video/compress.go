package video

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type compressPreset struct {
	videoBitrate  string
	audioBitrate  string
	audioChannels int
	nvencPreset   string
	fps           string
}

var compressPresets = map[string]compressPreset{
	"ultralight": {videoBitrate: "150k", audioBitrate: "32k", audioChannels: 1, nvencPreset: "p1", fps: "10"},
	"light":      {videoBitrate: "250k", audioBitrate: "48k", audioChannels: 1, nvencPreset: "p1", fps: "15"},
	"balanced":   {videoBitrate: "400k", audioBitrate: "64k", audioChannels: 1, nvencPreset: "p1", fps: "24"},
	"slideshow":  {videoBitrate: "500k", audioBitrate: "48k", audioChannels: 1, nvencPreset: "p4", fps: "1/20"},
}

func (s *Service) Compress(ctx context.Context, req CompressRequest) (CompressResponse, error) {
	if strings.TrimSpace(req.Path) == "" {
		return CompressResponse{}, errors.New("path is required")
	}
	output, out, err := s.compress(ctx, req)
	if err != nil {
		return CompressResponse{}, errors.New(strings.TrimSpace(string(out)) + ": " + err.Error())
	}
	return CompressResponse{Output: output}, nil
}

func (s *Service) compress(ctx context.Context, req CompressRequest) (string, []byte, error) {
	presetName := req.Preset
	if presetName == "" {
		presetName = "light"
	}
	preset, ok := compressPresets[presetName]
	if !ok {
		return "", nil, fmt.Errorf("unknown preset %q", presetName)
	}
	resolution := normalizeResolution(presetName, req.Resolution)
	fps := req.FPS
	if fps == "" {
		fps = preset.fps
	}
	output := compressOutputPath(req, resolution)
	args := compressArgs(req, preset, resolution, fps, output)
	out, err := s.runner.CombinedOutput(ctx, s.tools.FFmpeg, args...)
	if err != nil {
		_ = os.Remove(output)
		return output, out, err
	}
	if req.Overwrite {
		finalPath := strings.TrimSuffix(req.Path, filepath.Ext(req.Path)) + ".mp4"
		_ = os.Remove(req.Path)
		if err := os.Rename(output, finalPath); err != nil {
			return output, out, err
		}
		output = finalPath
	}
	return output, out, nil
}

func normalizeResolution(presetName string, resolution int) int {
	if presetName == "slideshow" && resolution == 240 {
		resolution = 0
	}
	if resolution == 0 {
		resolution = 240
	}
	if resolution < 0 {
		resolution = 0
	}
	return resolution
}

func compressOutputPath(req CompressRequest, resolution int) string {
	if req.Output != "" {
		return req.Output
	}
	if req.Overwrite {
		return strings.TrimSuffix(req.Path, filepath.Ext(req.Path)) + ".tmp_compress.mp4"
	}
	stem := req.TargetName
	if stem == "" {
		stem = strings.TrimSuffix(filepath.Base(req.Path), filepath.Ext(req.Path))
	}
	suffix := "_slideshow"
	if resolution > 0 {
		suffix = fmt.Sprintf("_%dp", resolution)
	}
	return filepath.Join(filepath.Dir(req.Path), stem+suffix+".mp4")
}

func compressArgs(req CompressRequest, preset compressPreset, resolution int, fps string, output string) []string {
	args := []string{"-y", "-v", "error", "-stats"}
	externalSRT := !req.SkipSubtitles && req.ExternalSRT != "" && fileExists(req.ExternalSRT)
	if resolution > 0 {
		args = append(args, "-hwaccel", "cuda", "-hwaccel_output_format", "cuda")
		if req.FastSkip {
			args = append(args, "-skip_frame", "nokey")
		}
		args = append(args, "-i", req.Path)
		if externalSRT {
			args = append(args, "-i", req.ExternalSRT)
		}
		args = append(args, "-vf", fmt.Sprintf("scale_cuda=-2:%d", resolution))
	} else {
		if req.FastSkip {
			args = append(args, "-hwaccel", "cuda", "-skip_frame", "nokey")
		}
		args = append(args, "-i", req.Path)
		if externalSRT {
			args = append(args, "-i", req.ExternalSRT)
		}
	}
	args = append(args, "-c:v", "h264_nvenc", "-preset", preset.nvencPreset, "-tune", "ll", "-r", fps)
	if req.CRF > 0 {
		args = append(args, "-cq", strconv.Itoa(req.CRF), "-b:v", "0")
	} else {
		args = append(args, "-b:v", preset.videoBitrate)
	}
	args = append(args, "-c:a", "aac", "-b:a", preset.audioBitrate, "-ac", strconv.Itoa(preset.audioChannels), "-ar", "22050")
	args = append(args, "-map", "0:v:0", "-map", "0:a:0?")
	if req.SkipSubtitles {
		args = append(args, "-sn")
	} else if externalSRT {
		args = append(args, "-map", "1:s?", "-c:s", "mov_text")
	} else {
		args = append(args, "-map", "0:s?", "-c:s", "mov_text")
	}
	args = append(args, "-map_metadata", "0", "-map_chapters", "0")
	if !req.SkipFastStart {
		args = append(args, "-movflags", "+faststart")
	}
	return append(args, output)
}
