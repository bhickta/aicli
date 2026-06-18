package lecture

import (
	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/tool"
)

type Service struct {
	provider    provider.Provider
	tools       config.ToolConfig
	runner      tool.Runner
	artifactDir string
}

type Request struct {
	Model           string `json:"model"`
	VaultPath       string `json:"vault_path"`
	SourcePath      string `json:"source_path"`
	OutputName      string `json:"output_name"`
	Style           string `json:"style"`
	MaxNotes        int    `json:"max_notes"`
	MaxInputChars   int    `json:"max_input_chars"`
	SynthesizeAudio bool   `json:"synthesize_audio"`
	TTSCommand      string `json:"tts_command"`
	TTSArgs         string `json:"tts_args"`
}

type Response struct {
	Kind           string   `json:"kind"`
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	Script         string   `json:"script"`
	ScriptPath     string   `json:"script_path"`
	ScriptURL      string   `json:"script_url"`
	AudioPath      string   `json:"audio_path,omitempty"`
	AudioURL       string   `json:"audio_url,omitempty"`
	SourceNotes    []string `json:"source_notes"`
	SkippedNotes   int      `json:"skipped_notes"`
	InputChars     int      `json:"input_chars"`
	TTSCommandLine []string `json:"tts_command_line,omitempty"`
}

type ProgressFunc func(stage string, completed int, total int, label string)

type Option func(*Service)

func New(provider provider.Provider, tools config.ToolConfig, runner tool.Runner, options ...Option) *Service {
	svc := &Service{provider: provider, tools: tools, runner: runner}
	for _, option := range options {
		option(svc)
	}
	return svc
}

func WithArtifactDir(path string) Option {
	return func(s *Service) {
		s.artifactDir = path
	}
}
