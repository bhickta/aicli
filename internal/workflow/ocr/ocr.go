package ocr

import (
	"context"
	"errors"
	"strings"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/tool"
	"github.com/bhickta/aicli/internal/workflow/document"
)

type Service struct {
	provider    provider.Provider
	tools       config.ToolConfig
	runner      tool.Runner
	artifactDir string
}

type Request struct {
	Model         string `json:"model"`
	Path          string `json:"path"`
	DPI           int    `json:"dpi"`
	RenderWorkers int    `json:"render_workers"`
	Workers       int    `json:"workers"`
}

type Page struct {
	Name     string `json:"name"`
	Markdown string `json:"markdown"`
	Path     string `json:"path,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

type Response struct {
	Markdown string `json:"markdown"`
	Pages    []Page `json:"pages"`
}

func New(provider provider.Provider, options ...Option) *Service {
	svc := &Service{provider: provider}
	for _, option := range options {
		option(svc)
	}
	return svc
}

type Option func(*Service)

func WithPDFRenderer(tools config.ToolConfig, runner tool.Runner) Option {
	return func(s *Service) {
		s.tools = tools
		s.runner = runner
	}
}

func WithArtifactDir(path string) Option {
	return func(s *Service) {
		s.artifactDir = path
	}
}

func (s *Service) Run(ctx context.Context, req Request) (Response, error) {
	if strings.TrimSpace(req.Path) == "" {
		return Response{}, errors.New("path is required")
	}
	inputs, err := zipImageInputs(req.Path)
	if err != nil {
		return Response{}, err
	}
	pages, err := s.ocrInputs(ctx, req, inputs)
	if err != nil {
		return Response{}, err
	}
	return s.responseFromDocumentPages(pages), nil
}

func (s *Service) ocrInputs(ctx context.Context, req Request, inputs []document.ImageInput) ([]document.OCRPage, error) {
	return document.OCRImages(
		ctx,
		s.provider,
		req.Model,
		inputs,
		"Extract all text from this page image as Markdown. Preserve headings, lists, tables, and reading order. Output Markdown only.",
		req.Workers,
		nil,
	)
}

func (s *Service) responseFromDocumentPages(pages []document.OCRPage) Response {
	out := make([]Page, 0, len(pages))
	for _, page := range pages {
		out = append(out, Page{
			Name:     page.Name,
			Markdown: page.Text,
			Path:     page.Path,
			ImageURL: artifactURL(s.artifactDir, page.Path),
		})
	}
	return Response{Markdown: document.AssembleMarkdown(pages), Pages: out}
}
