package analyze

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bhickta/aicli/internal/workflow/document"
)

func (s *Service) ReprocessReview(ctx context.Context, review Response, req ReprocessRequest, progress ProgressFunc) (Response, error) {
	if strings.TrimSpace(req.Model) == "" {
		return Response{}, fmt.Errorf("model is required")
	}
	action := strings.TrimSpace(req.Action)
	if action == "" {
		action = "all"
	}
	selected := selectedPageSet(req.PageNumbers, review.Pages)
	total := reprocessTotalUnits(action, len(selected))
	completed := 0
	progressUnits(progress, "loading saved topper copy review", completed, total, "stage")
	s.logInfo("topper copy reprocess started", "review_id", review.ReviewID, "action", action, "selected_pages", len(selected), "requested_ocr_workers", req.Workers, "requested_question_workers", req.QuestionWorkers)

	if action == "ocr" || action == "all" {
		stageStart := time.Now()
		if err := s.reprocessOCRPages(ctx, firstNonBlank(req.OCRModel, req.Model), &review, selected, req.Workers, req.OCRBatchSize, func(done int, totalPages int) {
			progressUnits(progress, fmt.Sprintf("rerunning OCR for %d page(s)", totalPages), completed+done, total, "page")
		}); err != nil {
			s.logWarn("topper copy reprocess OCR failed", "review_id", review.ReviewID, "selected_pages", len(selected), "elapsed_ms", elapsedMS(stageStart), "error", err)
			return Response{}, err
		}
		s.logInfo("topper copy reprocess OCR completed", "review_id", review.ReviewID, "selected_pages", len(selected), "elapsed_ms", elapsedMS(stageStart))
		completed += len(selected)
		progressUnits(progress, "page OCR updated", completed, total, "stage")
	}

	if action == "questions" || action == "all" {
		pages := answerBearingPages(pagesFromSet(review.Pages, selected))
		stageStart := time.Now()
		questions, err := s.splitQuestions(ctx, firstNonBlank(req.QuestionModel, req.Model), pages, req.QuestionWorkers, func(done int, totalPages int) {
			progressUnits(progress, fmt.Sprintf("splitting %d selected page(s)", totalPages), completed+done, total, "page")
		})
		if err != nil {
			s.logWarn("topper copy reprocess question split failed", "review_id", review.ReviewID, "pages", len(pages), "elapsed_ms", elapsedMS(stageStart), "error", err)
			return Response{}, err
		}
		s.logInfo("topper copy reprocess question split completed", "review_id", review.ReviewID, "pages", len(pages), "questions", len(questions), "elapsed_ms", elapsedMS(stageStart))

		stageStart = time.Now()
		questions = s.extractDimensions(ctx, firstNonBlank(req.QuestionModel, req.Model), questions, req.QuestionWorkers, func(done int, totalQ int) {
			// Optional: we can report progress without incrementing the global stage tracker until finished
		})
		s.logInfo("topper copy reprocess dimensions extracted", "review_id", review.ReviewID, "questions", len(questions), "elapsed_ms", elapsedMS(stageStart))

		review.Questions = replaceQuestionsForPages(review.Questions, questions, selected)
		completed += len(selected)
		progressUnits(progress, "question blocks updated", completed, total, "stage")
	}

	if action == "report" || action == "questions" || action == "all" {
		stageStart := time.Now()
		pages := answerBearingPages(review.Pages)
		questions := questionsForPages(review.Questions, pages)
		report, err := s.report(ctx, firstNonBlank(req.ReportModel, req.Model), pages, questions)
		if err != nil {
			s.logWarn("topper copy reprocess report failed", "review_id", review.ReviewID, "pages", len(pages), "questions", len(questions), "elapsed_ms", elapsedMS(stageStart), "error", err)
			return Response{}, err
		}
		s.logInfo("topper copy reprocess report completed", "review_id", review.ReviewID, "pages", len(pages), "questions", len(questions), "report_chars", len(report), "elapsed_ms", elapsedMS(stageStart))
		review.Report = report
		completed++
		progressUnits(progress, "final analysis updated", completed, total, "stage")
	}

	progressUnits(progress, "topper copy review updated", total, total, "stage")
	s.logInfo("topper copy reprocess completed", "review_id", review.ReviewID, "action", action)
	return review, nil
}

func (s *Service) reprocessOCRPages(ctx context.Context, model string, review *Response, selected map[int]bool, workers int, batchSize int, progress func(completed int, total int)) error {
	inputs := []document.ImageInput{}
	indexes := []int{}
	for index, page := range review.Pages {
		if !selected[page.Number] {
			continue
		}
		inputs = append(inputs, document.ImageInput{
			Name:     page.Name,
			Path:     page.Path,
			MIMEType: "image/jpeg",
		})
		indexes = append(indexes, index)
	}
	if len(inputs) == 0 {
		return nil
	}
	ocrPages, err := document.OCRImagesWithOptions(ctx, s.ocrProvider, model, inputs, topperCopyOCRPrompt, document.OCRImagesOptions{
		Workers:   workers,
		BatchSize: batchSize,
		Logger:    s.logger,
		Progress:  progress,
	})
	if err != nil {
		return err
	}
	for i, page := range ocrPages {
		target := indexes[i]
		review.Pages[target].Text = page.Text
		review.Pages[target].UnclearCount = strings.Count(strings.ToLower(page.Text), "[unclear]")
		review.Pages[target].Verified = false
	}
	return nil
}

func reprocessTotalUnits(action string, pages int) int {
	total := 1
	switch action {
	case "ocr":
		total = pages
	case "questions":
		total = pages + 1
	case "report":
		total = 1
	default:
		total = pages + pages + 1
	}
	if total < 1 {
		return 1
	}
	return total
}

func selectedPageSet(pageNumbers []int, pages []Page) map[int]bool {
	selected := map[int]bool{}
	for _, number := range pageNumbers {
		selected[number] = true
	}
	if len(selected) > 0 {
		return selected
	}
	for _, page := range pages {
		selected[page.Number] = true
	}
	return selected
}

func pagesFromSet(pages []Page, selected map[int]bool) []Page {
	out := []Page{}
	for _, page := range pages {
		if selected[page.Number] {
			out = append(out, page)
		}
	}
	return out
}

func replaceQuestionsForPages(existing []Question, next []Question, selected map[int]bool) []Question {
	merged := []Question{}
	for _, question := range existing {
		if questionTouchesSelectedPage(question, selected) {
			continue
		}
		merged = append(merged, question)
	}
	merged = append(merged, next...)
	sortQuestions(merged)
	return merged
}

func questionTouchesSelectedPage(question Question, selected map[int]bool) bool {
	for _, page := range question.SourcePages {
		if selected[page] {
			return true
		}
	}
	return false
}
