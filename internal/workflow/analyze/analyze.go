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
	svc := &Service{tools: tools, runner: runner, ocrProvider: provider, questionProvider: provider, reportProvider: provider}
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
	totalSteps := 3
	if req.QuestionSplit {
		totalSteps++
	}
	completedSteps := 0
	progressUnits(progress, "rendering PDF pages", completedSteps, totalSteps, "step")
	images, cleanup, err := document.RenderPDFToImages(ctx, s.tools, s.runner, req.Path, req.DPI, req.RenderWorkers)
	if err != nil {
		return Response{}, err
	}
	defer cleanup()
	completedSteps++
	progressUnits(progress, "rendered PDF pages", completedSteps, totalSteps, "step")

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
	ocrWorkers := document.EffectiveOCRWorkersForProvider(req.Workers, len(inputs), s.ocrProvider.ID())
	ocrPages, err := document.OCRImages(
		ctx,
		s.ocrProvider,
		firstNonBlank(req.OCRModel, req.Model),
		inputs,
		topperCopyOCRPrompt,
		req.Workers,
		func(completedPages int, totalPages int) {
			progressUnits(progress, fmt.Sprintf("OCR pages with %d worker(s)", ocrWorkers), completedPages, totalPages, "page")
		},
	)
	if err != nil {
		return Response{}, err
	}
	completedSteps++
	progressUnits(progress, "OCR pages complete", completedSteps, totalSteps, "step")
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
		questionWorkers := EffectiveQuestionWorkers(req.QuestionWorkers, len(pages))
		questions, err = s.splitQuestions(ctx, firstNonBlank(req.QuestionModel, req.Model), pages, req.QuestionWorkers, func(completedPages int, totalPages int) {
			progressUnits(progress, fmt.Sprintf("question-wise split with %d worker(s)", questionWorkers), completedPages, totalPages, "page")
		})
		if err != nil {
			return Response{}, err
		}
		completedSteps++
		progressUnits(progress, "question-wise split complete", completedSteps, totalSteps, "step")
	}
	progressUnits(progress, "generating final analysis", completedSteps, totalSteps, "step")
	report, err := s.report(ctx, firstNonBlank(req.ReportModel, req.Model), pages, questions)
	if err != nil {
		return Response{}, err
	}
	completedSteps++
	progressUnits(progress, "final analysis complete", completedSteps, totalSteps, "step")
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
	progressUnits(progress, "topper copy review ready", totalSteps, totalSteps, "step")
	return res, nil
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func progressUnits(progress ProgressFunc, stage string, completed int, total int, label string) {
	if progress != nil {
		progress(stage, completed, total, label)
	}
}
