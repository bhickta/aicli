package config

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
)

type Settings struct {
	DefaultProvider string           `json:"default_provider"`
	DefaultModel    string           `json:"default_model"`
	Providers       []ProviderConfig `json:"providers"`
	Tools           ToolConfig       `json:"tools"`
}

type ProviderConfig struct {
	ID      string            `json:"id"`
	Type    string            `json:"type"`
	Name    string            `json:"name"`
	BaseURL string            `json:"base_url"`
	APIKey  string            `json:"api_key"`
	Model   string            `json:"model"`
	Headers map[string]string `json:"headers"`
}

type ToolConfig struct {
	FFmpeg     string `json:"ffmpeg"`
	FFprobe    string `json:"ffprobe"`
	PDFToPPM   string `json:"pdftoppm"`
	WhisperCLI string `json:"whisper_cli"`
}

func DefaultDataDir() (string, error) {
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfg, "aicli"), nil
}

func DefaultSettings() Settings {
	return Settings{
		DefaultProvider: "lms",
		DefaultModel:    "",
		Providers: []ProviderConfig{
			{
				ID:      "lms",
				Type:    "openai-compatible",
				Name:    "LMS",
				BaseURL: "http://localhost:1234/v1",
				APIKey:  "lms",
			},
			{
				ID:      "ollama",
				Type:    "ollama",
				Name:    "Ollama",
				BaseURL: "http://localhost:11434",
			},
			{
				ID:      "openrouter",
				Type:    "openai-compatible",
				Name:    "OpenRouter",
				BaseURL: "https://openrouter.ai/api/v1",
			},
		},
		Tools: ToolConfig{
			FFmpeg:     "ffmpeg",
			FFprobe:    "ffprobe",
			PDFToPPM:   "pdftoppm",
			WhisperCLI: "whisper",
		},
	}
}

func Load(path string) (Settings, error) {
	if path == "" {
		return Settings{}, errors.New("config path is required")
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		settings := DefaultSettings()
		if err := Save(path, settings); err != nil {
			return Settings{}, err
		}
		return settings, nil
	}
	if err != nil {
		return Settings{}, err
	}

	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return Settings{}, err
	}
	return withDefaults(settings), nil
}

func Save(path string, settings Settings) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(withDefaults(settings), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func withDefaults(settings Settings) Settings {
	defaults := DefaultSettings()
	if settings.DefaultProvider == "" {
		settings.DefaultProvider = defaults.DefaultProvider
	}
	if settings.Providers == nil {
		settings.Providers = defaults.Providers
	}
	if settings.Tools.FFmpeg == "" {
		settings.Tools.FFmpeg = defaults.Tools.FFmpeg
	}
	if settings.Tools.FFprobe == "" {
		settings.Tools.FFprobe = defaults.Tools.FFprobe
	}
	if settings.Tools.PDFToPPM == "" {
		settings.Tools.PDFToPPM = defaults.Tools.PDFToPPM
	}
	if settings.Tools.WhisperCLI == "" {
		settings.Tools.WhisperCLI = defaults.Tools.WhisperCLI
	}
	if settings.Tools.WhisperCLI == "whisper-cli" {
		if _, err := exec.LookPath("whisper-cli"); err != nil {
			if _, fallbackErr := exec.LookPath("whisper"); fallbackErr == nil {
				settings.Tools.WhisperCLI = "whisper"
			}
		}
	}
	return settings
}
