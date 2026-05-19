package video

import (
	"context"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/tool"
	"github.com/bhickta/aicli/internal/workflow/video/info"
	"github.com/bhickta/aicli/internal/workflow/video/llm"
	"github.com/bhickta/aicli/internal/workflow/video/metadata"
)

type Service struct {
	tools    config.ToolConfig
	runner   tool.Runner
	provider provider.Provider
}

type InfoRequest = info.Request
type InfoResponse = info.Response

type CompressRequest struct {
	Path        string `json:"path"`
	Output      string `json:"output"`
	CRF         int    `json:"crf"`
	Resolution  int    `json:"resolution"`
	Preset      string `json:"preset"`
	Overwrite   bool   `json:"overwrite"`
	FPS         string `json:"fps"`
	FastSkip    bool   `json:"fast_skip"`
	ExternalSRT string `json:"external_srt"`
	TargetName  string `json:"target_name"`
}

type CompressResponse struct {
	Output string `json:"output"`
}

type CourseRequest struct {
	Path               string  `json:"path"`
	OutputDir          string  `json:"output_dir"`
	WhisperModel       string  `json:"whisper_model"`
	WhisperDevice      string  `json:"whisper_device"`
	Resolution         int     `json:"resolution"`
	Preset             string  `json:"preset"`
	CRF                int     `json:"crf"`
	FPS                string  `json:"fps"`
	FastSkip           bool    `json:"fast_skip"`
	Workers            int     `json:"workers"`
	TranscriptWorkers  int     `json:"transcript_workers"`
	CompressionWorkers int     `json:"compression_workers"`
	SkipUnreadable     bool    `json:"skip_unreadable"`
	MaxMergeHours      float64 `json:"max_merge_hours"`
}

type CourseResponse struct {
	CourseDir   string       `json:"course_dir"`
	VideoPath   string       `json:"video_path,omitempty"`
	SRTPath     string       `json:"srt_path,omitempty"`
	TextPath    string       `json:"text_path,omitempty"`
	Compressed  []CourseItem `json:"compressed"`
	Transcribed []CourseItem `json:"transcribed,omitempty"`
	Skipped     []string     `json:"skipped,omitempty"`
}

type CourseItem struct {
	Source     string `json:"source"`
	Output     string `json:"output"`
	SRTPath    string `json:"srt_path,omitempty"`
	TextPath   string `json:"text_path,omitempty"`
	TargetName string `json:"target_name"`
}

type CourseProgressFunc func(stage string, currentStep, totalSteps int)

type MetadataRequest = metadata.Request
type MetadataResponse = metadata.Response

type LLMRequest = llm.Request
type LLMResponse = llm.Response

func New(tools config.ToolConfig, runner tool.Runner, providers ...provider.Provider) *Service {
	var p provider.Provider
	if len(providers) > 0 {
		p = providers[0]
	}
	return &Service{tools: tools, runner: runner, provider: p}
}

func (s *Service) Info(ctx context.Context, req InfoRequest) (InfoResponse, error) {
	return info.New(s.tools, s.runner).Run(ctx, req)
}

func (s *Service) BackupMetadata(ctx context.Context, req MetadataRequest) (MetadataResponse, error) {
	return metadata.New(s.tools, s.runner).Backup(ctx, req)
}

func (s *Service) RestoreMetadata(ctx context.Context, req MetadataRequest) (MetadataResponse, error) {
	return metadata.New(s.tools, s.runner).Restore(ctx, req)
}

func (s *Service) Generate(ctx context.Context, req LLMRequest) (LLMResponse, error) {
	return llm.New(s.provider).Generate(ctx, req)
}
