package image

import (
	"context"
	"errors"
	"mime"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bhickta/aicli/internal/provider"
)

type Service struct {
	provider provider.Provider
}

type Request struct {
	Model string `json:"model"`
	Path  string `json:"path"`
	Mode  string `json:"mode"`
}

type Response struct {
	Text string `json:"text"`
}

type RenameRequest struct {
	Model string `json:"model"`
	Path  string `json:"path"`
	Apply bool   `json:"apply"`
}

type RenameResponse struct {
	OriginalPath string `json:"original_path"`
	NewPath      string `json:"new_path"`
	Stem         string `json:"stem"`
	Applied      bool   `json:"applied"`
}

type PruneRefsRequest struct {
	MarkdownPath string `json:"markdown_path"`
	AssetDir     string `json:"asset_dir"`
	Apply        bool   `json:"apply"`
}

type PruneRefsResponse struct {
	Removed []string `json:"removed"`
	Apply   bool     `json:"apply"`
}

func New(provider provider.Provider) *Service {
	return &Service{provider: provider}
}

func (s *Service) Run(ctx context.Context, req Request) (Response, error) {
	if strings.TrimSpace(req.Path) == "" {
		return Response{}, errors.New("path is required")
	}
	data, err := os.ReadFile(req.Path)
	if err != nil {
		return Response{}, err
	}

	prompt, err := promptForMode(req.Mode)
	if err != nil {
		return Response{}, err
	}
	res, err := s.provider.Vision(ctx, provider.VisionRequest{
		Model:       req.Model,
		Prompt:      prompt,
		Image:       data,
		MIMEType:    mime.TypeByExtension(strings.ToLower(filepath.Ext(req.Path))),
		Temperature: 0,
		MaxTokens:   1200,
	})
	if err != nil {
		return Response{}, err
	}
	return Response{Text: strings.TrimSpace(res.Content)}, nil
}

func (s *Service) Rename(ctx context.Context, req RenameRequest) (RenameResponse, error) {
	res, err := s.Run(ctx, Request{Model: req.Model, Path: req.Path, Mode: "rename"})
	if err != nil {
		return RenameResponse{}, err
	}
	stem := safeStem(res.Text)
	if stem == "" {
		return RenameResponse{}, errors.New("model did not return a safe filename")
	}
	newPath := filepath.Join(filepath.Dir(req.Path), stem+filepath.Ext(req.Path))
	out := RenameResponse{OriginalPath: req.Path, NewPath: newPath, Stem: stem, Applied: req.Apply}
	if !req.Apply || newPath == req.Path {
		return out, nil
	}
	if _, err := os.Stat(newPath); err == nil {
		return RenameResponse{}, errors.New("target file already exists")
	}
	if err := os.Rename(req.Path, newPath); err != nil {
		return RenameResponse{}, err
	}
	return out, nil
}

func PruneRefs(req PruneRefsRequest) (PruneRefsResponse, error) {
	if req.MarkdownPath == "" || req.AssetDir == "" {
		return PruneRefsResponse{}, errors.New("markdown_path and asset_dir are required")
	}
	data, err := os.ReadFile(req.MarkdownPath)
	if err != nil {
		return PruneRefsResponse{}, err
	}
	used := referencedAssets(string(data))
	entries, err := os.ReadDir(req.AssetDir)
	if err != nil {
		return PruneRefsResponse{}, err
	}
	removed := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if used[name] {
			continue
		}
		path := filepath.Join(req.AssetDir, name)
		removed = append(removed, path)
		if req.Apply {
			if err := os.Remove(path); err != nil {
				return PruneRefsResponse{}, err
			}
		}
	}
	return PruneRefsResponse{Removed: removed, Apply: req.Apply}, nil
}

func promptForMode(mode string) (string, error) {
	switch mode {
	case "", "rename":
		return "Suggest one safe descriptive filename for this image. Use lowercase words, hyphens, no extension, 3 to 8 words, and output only the filename stem.", nil
	case "junk":
		return "Classify whether this image is junk, duplicate-looking, accidental, blank, blurry, or useful. Output JSON with keys junk:boolean and reason:string.", nil
	case "digitize":
		return "Extract all visible text and structure from this image as clean Markdown. Preserve tables and lists. Output Markdown only.", nil
	default:
		return "", errors.New("unsupported image mode")
	}
}

func safeStem(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimSuffix(value, filepath.Ext(value))
	re := regexp.MustCompile(`[^a-z0-9]+`)
	value = re.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if len(value) > 96 {
		value = strings.Trim(value[:96], "-")
	}
	return value
}

func referencedAssets(markdown string) map[string]bool {
	re := regexp.MustCompile(`!\[[^\]]*\]\(([^)]+)\)`)
	matches := re.FindAllStringSubmatch(markdown, -1)
	used := map[string]bool{}
	for _, match := range matches {
		target := strings.Trim(match[1], `"' `)
		used[filepath.Base(target)] = true
	}
	return used
}
