package analyze

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/tool"
	"github.com/bhickta/aicli/internal/workflow/document"
)

type Service struct {
	tools    config.ToolConfig
	runner   tool.Runner
	provider provider.Provider
}

type Request struct {
	Model         string `json:"model"`
	Path          string `json:"path"`
	DPI           int    `json:"dpi"`
	RenderWorkers int    `json:"render_workers"`
	Workers       int    `json:"workers"`
}

type Page struct {
	Path string `json:"path"`
	Text string `json:"text"`
}

type Response struct {
	Pages  []Page `json:"pages"`
	Report string `json:"report"`
}

func New(tools config.ToolConfig, runner tool.Runner, provider provider.Provider) *Service {
	return &Service{tools: tools, runner: runner, provider: provider}
}

func (s *Service) Run(ctx context.Context, req Request) (Response, error) {
	if strings.TrimSpace(req.Path) == "" {
		return Response{}, errors.New("path is required")
	}
	images, cleanup, err := document.RenderPDFToImages(ctx, s.tools, s.runner, req.Path, req.DPI, req.RenderWorkers)
	if err != nil {
		return Response{}, err
	}
	defer cleanup()

	inputs := make([]document.ImageInput, 0, len(images))
	for i, imagePath := range images {
		inputs = append(inputs, document.ImageInput{
			Name:     "page-" + strconv.Itoa(i+1),
			Path:     imagePath,
			MIMEType: "image/jpeg",
		})
	}
	ocrPages, err := document.OCRImages(
		ctx,
		s.provider,
		req.Model,
		inputs,
		"Extract UPSC answer-script page text as Markdown. Preserve answer numbers and visible structure. Output Markdown only.",
		req.Workers,
	)
	if err != nil {
		return Response{}, err
	}
	pages := make([]Page, 0, len(ocrPages))
	for _, page := range ocrPages {
		pages = append(pages, Page{Path: page.Path, Text: page.Text})
	}

	report, err := s.report(ctx, req.Model, pages)
	if err != nil {
		return Response{}, err
	}
	return Response{Pages: pages, Report: report}, nil
}

func (s *Service) report(ctx context.Context, model string, pages []Page) (string, error) {
	var combined strings.Builder
	for i, page := range pages {
		combined.WriteString("## Page ")
		combined.WriteString(strconv.Itoa(i + 1))
		combined.WriteString("\n")
		combined.WriteString(page.Text)
		combined.WriteString("\n\n")
	}
	res, err := s.provider.Chat(ctx, provider.ChatRequest{
		Model: model,
		Messages: []provider.Message{
			{
				Role: "user",
				Content: "Analyze these UPSC answer pages. Segment answers, identify dimensions, aggregate strengths/weaknesses, and produce a concise Markdown report.\n\n" +
					combined.String(),
			},
		},
		Temperature: 0,
		MaxTokens:   4000,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(res.Content), nil
}
