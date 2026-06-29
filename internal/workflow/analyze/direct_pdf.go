package analyze

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	res, err := processor.Document(ctx, provider.DocumentRequest{
		Model:       firstNonBlank(req.OCRModel, req.Model),
		Prompt:      directPDFPrompt(filepath.Base(req.Path)),
		Data:        data,
		MIMEType:    "application/pdf",
		Temperature: 0,
		MaxTokens:   12000,
	})
	if err != nil {
		return Response{}, err
	}
	review, err := parseDirectPDFReview(res.Content, reviewID, filepath.Base(req.Path))
	if err != nil {
		return Response{}, err
	}
	return review, nil
}

func directPDFPrompt(pdfName string) string {
	return `Analyze this UPSC topper answer-copy PDF directly.

Return strict JSON only. Do not wrap in markdown fences.
Use this exact shape:
{
  "pages": [
    {"number": 1, "name": "page-1", "text": "brief source notes useful for inspection", "unclear_count": 0}
  ],
  "questions": [
    {
      "id": "q1",
      "label": "Q.1",
      "title": "visible question heading if present",
      "answer_markdown": "complete answer text transcribed from the PDF, preserving bullets, diagrams as text, examples, data, and [unclear] markers",
      "source_pages": [1, 2],
      "status": "detected"
    }
  ],
  "report": "Markdown final analysis with Executive Summary, Answer-Wise Analysis, Reusable Patterns, Weak Spots or Risks, and Action Checklist"
}

Rules:
- Extract final question/answer blocks directly from the full PDF.
- Preserve question numbers, page references, diagrams/flowcharts/maps as visible labels and arrows.
- Do not invent official model answers or facts not visible in the copy.
- Mark unreadable words as [unclear].
- Keep pages[].text concise; questions[].answer_markdown must contain the full visible answer text.
- The report must be based only on visible PDF content.

PDF name: ` + pdfName
}

func parseDirectPDFReview(content string, reviewID string, pdfName string) (Response, error) {
	jsonText, err := extractQuestionSplitJSON(content)
	if err != nil {
		return Response{}, err
	}
	var payload struct {
		Pages []struct {
			Number       int    `json:"number"`
			Name         string `json:"name"`
			Text         string `json:"text"`
			UnclearCount int    `json:"unclear_count"`
		} `json:"pages"`
		Questions []Question `json:"questions"`
		Report    string     `json:"report"`
	}
	if err := json.Unmarshal([]byte(jsonText), &payload); err != nil {
		return Response{}, err
	}
	if len(payload.Questions) == 0 {
		return Response{}, errors.New("direct PDF response returned no question blocks")
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
		answer := strings.TrimSpace(question.AnswerMarkdown)
		if answer == "" {
			continue
		}
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
		status := strings.TrimSpace(question.Status)
		if status == "" {
			status = "detected"
		}
		questions = append(questions, Question{
			ID:             id,
			Label:          label,
			Title:          strings.TrimSpace(question.Title),
			AnswerMarkdown: answer,
			SourcePages:    question.SourcePages,
			Status:         status,
		})
	}
	if len(questions) == 0 {
		return Response{}, errors.New("direct PDF response returned no usable question answers")
	}
	return Response{
		Kind:       "topper_copy_review",
		ReviewID:   reviewID,
		PDFName:    pdfName,
		SourceMode: OCRInputModePDFDirect,
		Pages:      pages,
		Questions:  questions,
		Report:     strings.TrimSpace(payload.Report),
	}, nil
}
