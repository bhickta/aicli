package analyze

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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
	workflowStart := time.Now()
	if strings.TrimSpace(req.Path) == "" {
		return Response{}, errors.New("path is required")
	}
	if req.DPI == 0 {
		req.DPI = defaultTopperCopyDPI
	}
	if req.QuestionWorkers < 0 {
		req.QuestionWorkers = 0
	}
	s.logInfo("topper copy analysis started",
		"path", req.Path,
		"dpi", req.DPI,
		"render_workers", req.RenderWorkers,
		"requested_ocr_workers", req.Workers,
		"requested_ocr_batch_size", req.OCRBatchSize,
		"question_split", req.QuestionSplit,
		"requested_question_workers", req.QuestionWorkers,
		"ocr_provider", providerID(s.ocrProvider),
		"question_provider", providerID(s.questionProvider),
		"report_provider", providerID(s.reportProvider),
		"ocr_model", firstNonBlank(req.OCRModel, req.Model),
		"question_model", firstNonBlank(req.QuestionModel, req.Model),
		"report_model", firstNonBlank(req.ReportModel, req.Model),
	)
	totalSteps := 3
	if req.QuestionSplit {
		totalSteps++
	}
	completedSteps := 0
	reviewIDValue := firstNonBlank(req.ReviewID, reviewID())
	reviewDir := ""
	pages := append([]Page(nil), req.OCRPages...)
	var err error
	var stageStart time.Time
	if len(pages) > 0 && !req.ForceOCR {
		progressUnits(progress, "using saved OCR pages", completedSteps, totalSteps, "step")
		s.logInfo("topper copy OCR reused", "path", req.Path, "review_id", reviewIDValue, "pages", len(pages))
		completedSteps++
		progressUnits(progress, "saved OCR pages loaded", completedSteps, totalSteps, "step")
	} else if s.shouldUseDirectPDF(req) {
		progressUnits(progress, "analyzing full PDF directly", completedSteps, totalSteps, "step")
		stageStart = time.Now()
		res, err := s.directPDFReview(ctx, req, reviewIDValue)
		if err != nil {
			s.logWarn("topper copy direct PDF analysis failed", "path", req.Path, "elapsed_ms", elapsedMS(stageStart), "error", err)
			return Response{}, err
		}
		progressUnits(progress, "topper copy review ready", totalSteps, totalSteps, "step")
		s.logInfo("topper copy direct PDF analysis completed", "path", req.Path, "review_id", reviewIDValue, "questions", len(res.Questions), "elapsed_ms", elapsedMS(workflowStart))
		return res, nil
	} else {
		progressUnits(progress, "rendering PDF pages", completedSteps, totalSteps, "step")
		stageStart := time.Now()
		images, cleanup, err := document.RenderPDFToImages(ctx, s.tools, s.runner, req.Path, req.DPI, req.RenderWorkers)
		if err != nil {
			s.logWarn("topper copy render failed", "path", req.Path, "elapsed_ms", elapsedMS(stageStart), "error", err)
			return Response{}, err
		}
		s.logInfo("topper copy render completed", "path", req.Path, "pages", len(images), "elapsed_ms", elapsedMS(stageStart))
		defer cleanup()
		completedSteps++
		progressUnits(progress, "rendered PDF pages", completedSteps, totalSteps, "step")

		if strings.TrimSpace(s.artifactDir) != "" {
			reviewDir = filepath.Join(s.artifactDir, "topper-copy", reviewIDValue)
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
		ocrWorkers := document.EffectiveOCRWorkersForVisionProvider(req.Workers, len(inputs), s.ocrProvider)
		ocrBatchSize := document.EffectiveOCRBatchSizeForVisionProvider(req.OCRBatchSize, s.ocrProvider)
		stageStart = time.Now()
		s.logInfo("topper copy OCR started",
			"path", req.Path,
			"pages", len(inputs),
			"workers", ocrWorkers,
			"batch_size", ocrBatchSize,
			"provider", providerID(s.ocrProvider),
			"model", firstNonBlank(req.OCRModel, req.Model),
		)
		ocrPages, err := document.OCRImagesWithOptions(
			ctx,
			s.ocrProvider,
			firstNonBlank(req.OCRModel, req.Model),
			inputs,
			topperCopyOCRPrompt,
			document.OCRImagesOptions{
				Workers:   req.Workers,
				BatchSize: req.OCRBatchSize,
				Logger:    s.logger,
				Progress: func(completedPages int, totalPages int) {
					progressUnits(progress, fmt.Sprintf("OCR pages with %d worker(s), batch %d", ocrWorkers, ocrBatchSize), completedPages, totalPages, "page")
				},
			},
		)
		if err != nil {
			s.logWarn("topper copy OCR failed", "path", req.Path, "pages", len(inputs), "workers", ocrWorkers, "batch_size", ocrBatchSize, "elapsed_ms", elapsedMS(stageStart), "error", err)
			return Response{}, err
		}
		s.logInfo("topper copy OCR completed", "path", req.Path, "pages", len(ocrPages), "workers", ocrWorkers, "batch_size", ocrBatchSize, "elapsed_ms", elapsedMS(stageStart))
		completedSteps++
		progressUnits(progress, "OCR pages complete", completedSteps, totalSteps, "step")
		pages = make([]Page, 0, len(ocrPages))
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
		if s.ocrCheckpoint != nil {
			checkpoint := Response{
				Kind:      "topper_copy_review",
				ReviewID:  reviewIDValue,
				PDFName:   filepath.Base(req.Path),
				Pages:     pages,
				Questions: pageFallbackQuestions(pages),
				Report:    "OCR checkpoint saved. Complete question split and report generation to finish analysis.",
			}
			if err := s.ocrCheckpoint(checkpoint); err != nil {
				return Response{}, err
			}
			s.logInfo("topper copy OCR checkpoint saved", "path", req.Path, "review_id", reviewIDValue, "pages", len(pages))
		}
	}
	analysisPages := answerBearingPages(pages)
	s.logInfo("topper copy analysis page filter completed", "path", req.Path, "total_pages", len(pages), "analysis_pages", len(analysisPages), "skipped_pages", len(pages)-len(analysisPages))

	questions := pageFallbackQuestions(analysisPages)
	if req.QuestionSplit {
		questionWorkers := EffectiveQuestionWorkers(req.QuestionWorkers, len(analysisPages))
		stageStart = time.Now()
		s.logInfo("topper copy question split started", "path", req.Path, "pages", len(analysisPages), "workers", questionWorkers, "provider", providerID(s.questionProvider), "model", firstNonBlank(req.QuestionModel, req.Model))
		questions, err = s.splitQuestions(ctx, firstNonBlank(req.QuestionModel, req.Model), analysisPages, req.QuestionWorkers, func(completedPages int, totalPages int) {
			progressUnits(progress, fmt.Sprintf("question-wise split with %d worker(s)", questionWorkers), completedPages, totalPages, "page")
		})
		if err != nil {
			s.logWarn("topper copy question split failed", "path", req.Path, "pages", len(analysisPages), "workers", questionWorkers, "elapsed_ms", elapsedMS(stageStart), "error", err)
			return Response{}, err
		}
		s.logInfo("topper copy question split completed", "path", req.Path, "pages", len(analysisPages), "questions", len(questions), "workers", questionWorkers, "elapsed_ms", elapsedMS(stageStart))
		completedSteps++
		progressUnits(progress, "question-wise split complete", completedSteps, totalSteps, "step")
	}
	progressUnits(progress, "generating final analysis", completedSteps, totalSteps, "step")
	stageStart = time.Now()
	s.logInfo("topper copy report started", "path", req.Path, "pages", len(analysisPages), "questions", len(questions), "provider", providerID(s.reportProvider), "model", firstNonBlank(req.ReportModel, req.Model))
	report, err := s.report(ctx, firstNonBlank(req.ReportModel, req.Model), analysisPages, questions)
	if err != nil {
		s.logWarn("topper copy report failed", "path", req.Path, "pages", len(analysisPages), "questions", len(questions), "elapsed_ms", elapsedMS(stageStart), "error", err)
		return Response{}, err
	}
	s.logInfo("topper copy report completed", "path", req.Path, "pages", len(analysisPages), "questions", len(questions), "report_chars", len(report), "elapsed_ms", elapsedMS(stageStart))
	completedSteps++
	progressUnits(progress, "final analysis complete", completedSteps, totalSteps, "step")
	res := Response{
		Kind:      "topper_copy_review",
		ReviewID:  reviewIDValue,
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
	s.logInfo("topper copy analysis completed", "path", req.Path, "review_id", reviewIDValue, "pages", len(pages), "questions", len(questions), "elapsed_ms", elapsedMS(workflowStart))
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
