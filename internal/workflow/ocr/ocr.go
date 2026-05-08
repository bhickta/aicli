package ocr

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/tool"
	"github.com/bhickta/aicli/internal/workflow/document"
)

type Service struct {
	provider provider.Provider
	tools    config.ToolConfig
	runner   tool.Runner
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

func (s *Service) Run(ctx context.Context, req Request) (Response, error) {
	if strings.TrimSpace(req.Path) == "" {
		return Response{}, errors.New("path is required")
	}
	reader, err := zip.OpenReader(req.Path)
	if err != nil {
		return Response{}, err
	}
	defer reader.Close()

	files := imageFiles(reader.File)
	inputs := make([]document.ImageInput, 0, len(files))
	for _, file := range files {
		data, err := readZipFile(file)
		if err != nil {
			return Response{}, err
		}
		inputs = append(inputs, document.ImageInput{
			Name:     file.Name,
			Data:     data,
			MIMEType: mimeForName(file.Name),
		})
	}
	pages, err := s.ocrInputs(ctx, req, inputs)
	if err != nil {
		return Response{}, err
	}
	return responseFromDocumentPages(pages), nil
}

func (s *Service) RunPDF(ctx context.Context, req Request) (Response, error) {
	images, cleanup, err := document.RenderPDFToImages(ctx, s.tools, s.runner, req.Path, req.DPI, req.RenderWorkers)
	if err != nil {
		return Response{}, err
	}
	defer cleanup()

	inputs := make([]document.ImageInput, 0, len(images))
	for i, imagePath := range images {
		inputs = append(inputs, document.ImageInput{
			Name:     "page-" + itoa(i+1),
			Path:     imagePath,
			MIMEType: "image/jpeg",
		})
	}
	pages, err := s.ocrInputs(ctx, req, inputs)
	if err != nil {
		return Response{}, err
	}
	return responseFromDocumentPages(pages), nil
}

func (s *Service) ocrInputs(ctx context.Context, req Request, inputs []document.ImageInput) ([]document.OCRPage, error) {
	return document.OCRImages(
		ctx,
		s.provider,
		req.Model,
		inputs,
		"Extract all text from this page image as Markdown. Preserve headings, lists, tables, and reading order. Output Markdown only.",
		req.Workers,
	)
}

func responseFromDocumentPages(pages []document.OCRPage) Response {
	out := make([]Page, 0, len(pages))
	for _, page := range pages {
		out = append(out, Page{Name: page.Name, Markdown: page.Text})
	}
	return Response{Markdown: document.AssembleMarkdown(pages), Pages: out}
}

func imageFiles(files []*zip.File) []*zip.File {
	out := []*zip.File{}
	for _, file := range files {
		if file.FileInfo().IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(file.Name))
		if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".webp" {
			out = append(out, file)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return naturalLess(out[i].Name, out[j].Name)
	})
	return out
}

func readZipFile(file *zip.File) ([]byte, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(io.LimitReader(reader, 50<<20))
}

func mimeForName(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	default:
		return "image/jpeg"
	}
}

func naturalLess(a string, b string) bool {
	return strings.ToLower(a) < strings.ToLower(b)
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := []byte{}
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}
