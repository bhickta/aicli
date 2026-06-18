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
	ID                   string            `json:"id"`
	Type                 string            `json:"type"`
	Name                 string            `json:"name"`
	BaseURL              string            `json:"base_url"`
	APIKey               string            `json:"api_key"`
	APIKeyEnv            string            `json:"api_key_env,omitempty"`
	Model                string            `json:"model"`
	ModelFilter          string            `json:"model_filter,omitempty"`
	ReasoningEffort      string            `json:"reasoning_effort,omitempty"`
	TextVerbosity        string            `json:"text_verbosity,omitempty"`
	PromptCacheKey       string            `json:"prompt_cache_key,omitempty"`
	PromptCacheRetention string            `json:"prompt_cache_retention,omitempty"`
	Headers              map[string]string `json:"headers"`
}

type ToolConfig struct {
	FFmpeg     string `json:"ffmpeg"`
	FFprobe    string `json:"ffprobe"`
	PDFToPPM   string `json:"pdftoppm"`
	WhisperCLI string `json:"whisper_cli"`
	CodexCLI   string `json:"codex_cli"`
	GeminiCLI  string `json:"gemini_cli"`
	OTSTTS     string `json:"ots_tts"`
	OTSTTSArgs string `json:"ots_tts_args"`
	Firefox    string `json:"firefox"`
	XDoTool    string `json:"xdotool"`
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
			{
				ID:              "codex",
				Type:            "openai-responses",
				Name:            "OpenAI Codex",
				BaseURL:         "https://api.openai.com/v1",
				APIKeyEnv:       "OPENAI_API_KEY",
				Model:           "gpt-5.2-codex",
				ModelFilter:     "codex",
				ReasoningEffort: "medium",
				TextVerbosity:   "low",
				PromptCacheKey:  "aicli-codex",
			},
			{
				ID:              "codex-cli",
				Type:            "codex-cli",
				Name:            "Codex CLI / Pro",
				Model:           "gpt-5.5",
				ReasoningEffort: "medium",
				TextVerbosity:   "low",
			},
			{
				ID:   "gemini-cli",
				Type: "gemini-cli",
				Name: "Gemini CLI",
			},
		},
		Tools: ToolConfig{
			FFmpeg:     "ffmpeg",
			FFprobe:    "ffprobe",
			PDFToPPM:   "pdftoppm",
			WhisperCLI: "whisper",
			CodexCLI:   "codex",
			GeminiCLI:  "gemini",
			OTSTTS:     "ots.TTS",
			OTSTTSArgs: `SOAR --input "{script}" --output "{audio}"`,
			Firefox:    "firefox",
			XDoTool:    "xdotool",
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
	return Normalize(settings), nil
}

func Save(path string, settings Settings) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(Normalize(settings), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func Normalize(settings Settings) Settings {
	return withDefaults(settings)
}

func withDefaults(settings Settings) Settings {
	defaults := DefaultSettings()
	if settings.DefaultProvider == "" {
		settings.DefaultProvider = defaults.DefaultProvider
	}
	if settings.Providers == nil {
		settings.Providers = defaults.Providers
	} else {
		settings.Providers = mergeDefaultProviders(settings.Providers, defaults.Providers)
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
	if settings.Tools.CodexCLI == "" {
		settings.Tools.CodexCLI = defaults.Tools.CodexCLI
	}
	if settings.Tools.GeminiCLI == "" {
		settings.Tools.GeminiCLI = defaults.Tools.GeminiCLI
	}
	if settings.Tools.OTSTTS == "" {
		settings.Tools.OTSTTS = defaults.Tools.OTSTTS
	}
	if settings.Tools.OTSTTSArgs == "" {
		settings.Tools.OTSTTSArgs = defaults.Tools.OTSTTSArgs
	}
	if settings.Tools.Firefox == "" {
		settings.Tools.Firefox = defaults.Tools.Firefox
	}
	if settings.Tools.XDoTool == "" {
		settings.Tools.XDoTool = defaults.Tools.XDoTool
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

func mergeDefaultProviders(providers []ProviderConfig, defaults []ProviderConfig) []ProviderConfig {
	seen := make(map[string]bool, len(providers))
	defaultByID := make(map[string]ProviderConfig, len(defaults))
	for _, provider := range defaults {
		if provider.ID != "" {
			defaultByID[provider.ID] = provider
		}
	}
	for i, provider := range providers {
		if defaults, ok := defaultByID[provider.ID]; ok {
			providers[i] = mergeProviderDefaults(provider, defaults)
		}
		if provider.ID != "" {
			seen[provider.ID] = true
		}
	}
	for _, provider := range defaults {
		if provider.ID == "" || seen[provider.ID] {
			continue
		}
		providers = append(providers, provider)
		seen[provider.ID] = true
	}
	return providers
}

func mergeProviderDefaults(provider ProviderConfig, defaults ProviderConfig) ProviderConfig {
	if provider.Type == "" {
		provider.Type = defaults.Type
	}
	if provider.Name == "" {
		provider.Name = defaults.Name
	}
	if provider.BaseURL == "" {
		provider.BaseURL = defaults.BaseURL
	}
	if provider.APIKeyEnv == "" {
		provider.APIKeyEnv = defaults.APIKeyEnv
	}
	if provider.Model == "" {
		provider.Model = defaults.Model
	}
	if provider.ModelFilter == "" {
		provider.ModelFilter = defaults.ModelFilter
	}
	if provider.ReasoningEffort == "" {
		provider.ReasoningEffort = defaults.ReasoningEffort
	}
	if provider.TextVerbosity == "" {
		provider.TextVerbosity = defaults.TextVerbosity
	}
	if provider.PromptCacheKey == "" {
		provider.PromptCacheKey = defaults.PromptCacheKey
	}
	if provider.PromptCacheRetention == "" {
		provider.PromptCacheRetention = defaults.PromptCacheRetention
	}
	return provider
}
