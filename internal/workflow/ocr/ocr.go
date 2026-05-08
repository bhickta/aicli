package ocr

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bhickta/aicli/internal/provider"
)

type Service struct {
	provider provider.Provider
}

type Request struct {
	Model string `json:"model"`
	Path  string `json:"path"`
}

type Page struct {
	Name     string `json:"name"`
	Markdown string `json:"markdown"`
}

type Response struct {
	Markdown string `json:"markdown"`
	Pages    []Page `json:"pages"`
}

func New(provider provider.Provider) *Service {
	return &Service{provider: provider}
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
	pages := make([]Page, 0, len(files))
	for _, file := range files {
		data, err := readZipFile(file)
		if err != nil {
			return Response{}, err
		}
		res, err := s.provider.Vision(ctx, provider.VisionRequest{
			Model:       req.Model,
			Prompt:      "Extract all text from this page image as Markdown. Preserve headings, lists, tables, and reading order. Output Markdown only.",
			Image:       data,
			MIMEType:    mimeForName(file.Name),
			Temperature: 0,
			MaxTokens:   1800,
		})
		if err != nil {
			return Response{}, err
		}
		pages = append(pages, Page{Name: file.Name, Markdown: strings.TrimSpace(res.Content)})
	}

	parts := make([]string, 0, len(pages))
	for i, page := range pages {
		parts = append(parts, "<!-- Page "+itoa(i+1)+" "+page.Name+" -->\n"+page.Markdown)
	}
	return Response{Markdown: strings.Join(parts, "\n\n---\n\n"), Pages: pages}, nil
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
