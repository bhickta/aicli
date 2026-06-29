package analyze

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/bhickta/aicli/internal/provider"
)

const (
	OCRInputModeAuto      = "auto"
	OCRInputModeImages    = "images"
	OCRInputModePDFDirect = "pdf_direct"
)

func (s *Service) shouldUseDirectPDF(req Request) bool {
	mode := strings.ToLower(strings.TrimSpace(req.OCRInputMode))
	switch mode {
	case OCRInputModePDFDirect:
		return true
	case "", OCRInputModeAuto:
		return strings.Contains(strings.ToLower(providerID(s.ocrProvider)), "gemini")
	default:
		return false
	}
}

func (s *Service) directPDFReview(ctx context.Context, req Request, reviewID string) (Response, error) {
	processor, ok := s.ocrProvider.(provider.DocumentProcessor)
	if !ok {
		return Response{}, fmt.Errorf("OCR provider %q does not support direct PDF input; choose Page images mode", providerID(s.ocrProvider))
	}
	data, err := os.ReadFile(req.Path)
	if err != nil {
		return Response{}, err
	}

	s.logInfo("direct PDF manifest extraction started", "path", req.Path)
	manifestRes, err := processor.Document(ctx, provider.DocumentRequest{
		Model:       firstNonBlank(req.OCRModel, req.Model),
		Prompt:      directPDFManifestPrompt(filepath.Base(req.Path)),
		Data:        data,
		MIMEType:    "application/pdf",
		Temperature: 0,
		MaxTokens:   4000,
	})
	if err != nil {
		return Response{}, err
	}

	pages, questions, err := parseDirectPDFManifest(manifestRes.Content, filepath.Base(req.Path))
	if err != nil {
		return Response{}, err
	}

	s.logInfo("direct PDF manifest extracted", "pages", len(pages), "questions", len(questions))

	if len(questions) == 0 {
		return Response{}, errors.New("direct PDF manifest returned no question blocks")
	}

	// Extract answers for each question in parallel
	workers := EffectiveQuestionWorkers(req.QuestionWorkers, len(questions))
	s.logInfo("direct PDF answer extraction started", "questions", len(questions), "workers", workers)

	type job struct {
		index int
		q     Question
	}
	type result struct {
		index  int
		answer string
		err    error
	}

	jobs := make(chan job, len(questions))
	results := make(chan result, len(questions))

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				ans, err := s.extractDirectPDFAnswer(ctx, processor, firstNonBlank(req.OCRModel, req.Model), data, j.q)
				results <- result{index: j.index, answer: ans, err: err}
			}
		}()
	}

	for i, q := range questions {
		jobs <- job{index: i, q: q}
	}
	close(jobs)

	wg.Wait()
	close(results)

	// Collect results
	for res := range results {
		if res.err != nil {
			s.logWarn("direct PDF answer extraction failed for question", "question", questions[res.index].Label, "error", res.err)
			return Response{}, fmt.Errorf("failed to extract answer for %s: %w", questions[res.index].Label, res.err)
		}
		questions[res.index].AnswerMarkdown = res.answer
		questions[res.index].Status = "detected"
	}

	s.logInfo("direct PDF answer extraction completed", "questions", len(questions))

	// Generate report using the existing report generator
	s.logInfo("direct PDF report generation started")
	report, err := s.report(ctx, firstNonBlank(req.ReportModel, req.Model), pages, questions)
	if err != nil {
		s.logWarn("direct PDF report generation failed", "error", err)
		report = "Failed to generate report: " + err.Error()
	} else {
		s.logInfo("direct PDF report generation completed", "report_chars", len(report))
	}

	return Response{
		Kind:       "topper_copy_review",
		ReviewID:   reviewID,
		PDFName:    filepath.Base(req.Path),
		SourceMode: OCRInputModePDFDirect,
		Pages:      pages,
		Questions:  questions,
		Report:     report,
	}, nil
}

func directPDFManifestPrompt(pdfName string) string {
	return `Analyze this UPSC topper answer-copy PDF directly.
Identify all pages and all questions.

Return strict JSON only. Do not wrap in markdown fences.
Use this exact shape:
{
  "pages": [
    {"number": 1, "name": "page-1", "text": "brief summary of page content (1-2 sentences)", "unclear_count": 0}
  ],
  "questions": [
    {
      "id": "q1",
      "label": "Q.1",
      "title": "visible question heading if present (the actual question prompt text)",
      "source_pages": [1, 2]
    }
  ]
}

Rules:
- List every page in the PDF in the "pages" array.
- Identify every question block in the PDF. Each question should have a "label" (e.g., "Q.1", "Q.2(a)", etc.), a "title" (the exact text of the question prompt as written in the PDF), and "source_pages" (the 1-indexed page numbers where the question and its answer are written).
- Do not transcribe the full answer text yet. Just identify the questions, their titles, and their page ranges.

PDF name: ` + pdfName
}

func directPDFAnswerPrompt(q Question) string {
	return fmt.Sprintf(`Analyze this UPSC topper answer-copy PDF directly.
Focus ONLY on the question: %q (%s) which is located on pages: %v.

Extract and transcribe the complete, full answer text written by the candidate for this question.
Preserve the exact structure, bullets, diagrams as text, examples, data, and mark any unreadable words as [unclear].
Do not summarize. Do not invent official model answers or facts not visible in the copy.

Return strict JSON only. Do not wrap in markdown fences.
Use this exact shape:
{
  "answer_markdown": "complete answer text transcribed from the PDF"
}`, q.Label, q.Title, q.SourcePages)
}

func parseDirectPDFManifest(content string, pdfName string) ([]Page, []Question, error) {
	jsonText, err := extractQuestionSplitJSON(content)
	if err != nil {
		return nil, nil, err
	}
	var payload struct {
		Pages []struct {
			Number       int    `json:"number"`
			Name         string `json:"name"`
			Text         string `json:"text"`
			UnclearCount int    `json:"unclear_count"`
		} `json:"pages"`
		Questions []struct {
			ID          string `json:"id"`
			Label       string `json:"label"`
			Title       string `json:"title"`
			SourcePages []int  `json:"source_pages"`
		} `json:"questions"`
	}
	if err := json.Unmarshal([]byte(jsonText), &payload); err != nil {
		return nil, nil, err
	}

	pages := make([]Page, 0, len(payload.Pages))
	for i, page := range payload.Pages {
		number := page.Number
		if number <= 0 {
			number = i + 1
		}
		name := strings.TrimSpace(page.Name)
		if name == "" {
			name = fmt.Sprintf("page-%d", number)
		}
		pages = append(pages, Page{
			Number:       number,
			Name:         name,
			Text:         strings.TrimSpace(page.Text),
			UnclearCount: page.UnclearCount,
			Verified:     false,
		})
	}

	questions := make([]Question, 0, len(payload.Questions))
	for _, question := range payload.Questions {
		label := strings.TrimSpace(question.Label)
		if label == "" {
			label = strings.TrimSpace(question.ID)
		}
		if label == "" {
			label = fmt.Sprintf("Question %d", len(questions)+1)
		}
		id := strings.TrimSpace(question.ID)
		if id == "" {
			id = normalizeQuestionLabel(label)
		}
		questions = append(questions, Question{
			ID:          id,
			Label:       label,
			Title:       strings.TrimSpace(question.Title),
			SourcePages: question.SourcePages,
			Status:      "detected",
		})
	}

	return pages, questions, nil
}

func (s *Service) extractDirectPDFAnswer(ctx context.Context, processor provider.DocumentProcessor, model string, pdfData []byte, q Question) (string, error) {
	prompt := directPDFAnswerPrompt(q)
	res, err := processor.Document(ctx, provider.DocumentRequest{
		Model:       model,
		Prompt:      prompt,
		Data:        pdfData,
		MIMEType:    "application/pdf",
		Temperature: 0,
		MaxTokens:   8000,
	})
	if err != nil {
		return "", err
	}
	jsonText, err := extractQuestionSplitJSON(res.Content)
	if err != nil {
		return "", err
	}
	var payload struct {
		AnswerMarkdown string `json:"answer_markdown"`
	}
	if err := json.Unmarshal([]byte(jsonText), &payload); err != nil {
		return "", err
	}
	return strings.TrimSpace(payload.AnswerMarkdown), nil
}
