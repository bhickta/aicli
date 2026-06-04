package analyze

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/tool"
	"github.com/bhickta/aicli/internal/workflow/document"
)

const defaultTopperCopyDPI = 300

func New(tools config.ToolConfig, runner tool.Runner, provider provider.Provider, options ...Option) *Service {
	svc := &Service{tools: tools, runner: runner, provider: provider}
	for _, option := range options {
		option(svc)
	}
	return svc
}

func (s *Service) Run(ctx context.Context, req Request) (Response, error) {
	return s.RunWithProgress(ctx, req, nil)
}

func (s *Service) RunWithProgress(ctx context.Context, req Request, progress ProgressFunc) (Response, error) {
	if strings.TrimSpace(req.Path) == "" {
		return Response{}, errors.New("path is required")
	}
	if req.DPI == 0 {
		req.DPI = defaultTopperCopyDPI
	}
	if req.QuestionWorkers < 0 {
		req.QuestionWorkers = 0
	}
	progressUnits(progress, "rendering PDF pages", 0, 1, "stage")
	images, cleanup, err := document.RenderPDFToImages(ctx, s.tools, s.runner, req.Path, req.DPI, req.RenderWorkers)
	if err != nil {
		return Response{}, err
	}
	defer cleanup()
	total := 2 + len(images)
	if req.QuestionSplit {
		total += len(images)
	}
	completed := 1
	progressUnits(progress, "rendered PDF pages", completed, total, "stage")

	reviewID := reviewID()
	reviewDir := ""
	if strings.TrimSpace(s.artifactDir) != "" {
		reviewDir = filepath.Join(s.artifactDir, "topper-copy", reviewID)
		if err := os.MkdirAll(reviewDir, 0o755); err != nil {
			return Response{}, err
		}
	}

	inputs := make([]document.ImageInput, 0, len(images))
	for i, imagePath := range images {
		stablePath := imagePath
		if reviewDir != "" {
			stablePath = filepath.Join(reviewDir, fmt.Sprintf("page-%03d.jpg", i+1))
			if err := copyFile(stablePath, imagePath); err != nil {
				return Response{}, err
			}
		}
		inputs = append(inputs, document.ImageInput{
			Name:     "page-" + strconv.Itoa(i+1),
			Path:     stablePath,
			MIMEType: "image/jpeg",
		})
	}
	ocrPages, err := document.OCRImages(
		ctx,
		s.provider,
		req.Model,
		inputs,
		topperCopyOCRPrompt,
		req.Workers,
		func(completedPages int, totalPages int) {
			progressUnits(progress, "OCR pages", completed+completedPages, total, "stage")
		},
	)
	if err != nil {
		return Response{}, err
	}
	completed += len(images)
	progressUnits(progress, "OCR pages complete", completed, total, "stage")
	pages := make([]Page, 0, len(ocrPages))
	for i, page := range ocrPages {
		pages = append(pages, Page{
			Number:       i + 1,
			Name:         page.Name,
			Path:         page.Path,
			ImageURL:     artifactURL(s.artifactDir, page.Path),
			Text:         page.Text,
			UnclearCount: strings.Count(strings.ToLower(page.Text), "[unclear]"),
			Verified:     false,
		})
	}

	questions := pageFallbackQuestions(pages)
	if req.QuestionSplit {
		questions, err = s.splitQuestions(ctx, req.Model, pages, req.QuestionWorkers, func(completedPages int, totalPages int) {
			progressUnits(progress, "question-wise split", completed+completedPages, total, "stage")
		})
		if err != nil {
			return Response{}, err
		}
		completed += len(pages)
		progressUnits(progress, "question-wise split complete", completed, total, "stage")
	}
	progressUnits(progress, "generating final analysis", completed, total, "stage")
	report, err := s.report(ctx, req.Model, pages, questions)
	if err != nil {
		return Response{}, err
	}
	completed++
	progressUnits(progress, "final analysis complete", completed, total, "stage")
	res := Response{
		Kind:      "topper_copy_review",
		ReviewID:  reviewID,
		PDFName:   filepath.Base(req.Path),
		Pages:     pages,
		Questions: questions,
		Report:    report,
	}
	if reviewDir != "" {
		if err := writeReview(filepath.Join(reviewDir, "review.json"), res); err != nil {
			return Response{}, err
		}
	}
	progressUnits(progress, "topper copy review ready", total, total, "stage")
	return res, nil
}

func progressUnits(progress ProgressFunc, stage string, completed int, total int, label string) {
	if progress != nil {
		progress(stage, completed, total, label)
	}
}
