package video

import (
	"encoding/json"

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
	Path          string  `json:"path"`
	OutputDir     string  `json:"output_dir"`
	WhisperModel  string  `json:"whisper_model"`
	Resolution    int     `json:"resolution"`
	Preset        string  `json:"preset"`
	CRF           int     `json:"crf"`
	FPS           string  `json:"fps"`
	FastSkip      bool    `json:"fast_skip"`
	Workers       int     `json:"workers"`
	MaxMergeHours float64 `json:"max_merge_hours"`
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
