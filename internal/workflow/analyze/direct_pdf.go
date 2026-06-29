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

	s.logInfo("direct PDF one-shot extraction started", "path", req.Path)
	res, err := processor.Document(ctx, provider.DocumentRequest{
		Model:       firstNonBlank(req.OCRModel, req.Model),
		Prompt:      oneShotPDFPrompt(filepath.Base(req.Path)),
		Data:        data,
		MIMEType:    "application/pdf",
		Temperature: 0,
		MaxTokens:   8192,
	})
	if err != nil {
		return Response{}, err
	}

	pages, questions, report, err := parseOneShotPDFManifest(res.Content, filepath.Base(req.Path))
	if err != nil {
		return Response{}, err
	}

	s.logInfo("direct PDF one-shot extraction completed", "pages", len(pages), "questions", len(questions), "report_chars", len(report))

	if len(questions) == 0 {
		return Response{}, errors.New("direct PDF one-shot extraction returned no question blocks")
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

func oneShotPDFPrompt(pdfName string) string {
	return `Analyze this entire UPSC topper answer-copy PDF.
You must extract every page, every question (including the full transcribed answer text), evaluate analytical dimensions for each question, and finally provide a comprehensive overarching report.

You must return a valid JSON object matching the exact schema below.
- Do not wrap the JSON in markdown code fences (like ` + "```" + `json).
- Do not include any trailing commas.
- Escape all double quotes in string values as \".
- Escape all newlines in string values as \n.

Schema:
{
  "pages": [
    {"number": 1, "name": "page-1", "text": "brief summary of page content", "unclear_count": 0}
  ],
  "questions": [
    {
      "id": "q1",
      "label": "Q.1",
      "title": "exact question prompt text",
      "source_pages": [1, 2],
      "answer_markdown": "Full transcribed answer text...",
      "dimensions": {
        "introduction": "...",
        "outro": "...",
        "transition": "...",
        "diagram": "...",
        "fact": "...",
        "fact_usage": "...",
        "custom": "..."
      }
    }
  ],
  "report": "Comprehensive overarching analysis report of the entire copy..."
}

Rules:
1. "pages": List every page in the PDF.
2. "questions": Extract EVERY question. Provide the full transcription of the student's answer in "answer_markdown", preserving all structures, bullets, and text.
3. "dimensions": For each question, provide the analytical dimensions as evaluated based on their answer.
4. "report": A comprehensive overarching markdown report analyzing their strengths, weaknesses, and overall performance across the entire copy.

PDF name: ` + pdfName
}

func parseOneShotPDFManifest(content string, pdfName string) ([]Page, []Question, string, error) {
	jsonText, err := extractQuestionSplitJSON(content)
	if err != nil {
		return nil, nil, "", err
	}
	var payload struct {
		Pages []struct {
			Number       int    `json:"number"`
			Name         string `json:"name"`
			Text         string `json:"text"`
			UnclearCount int    `json:"unclear_count"`
		} `json:"pages"`
		Questions []struct {
			ID             string             `json:"id"`
			Label          string             `json:"label"`
			Title          string             `json:"title"`
			SourcePages    []int              `json:"source_pages"`
			AnswerMarkdown string             `json:"answer_markdown"`
			Dimensions     QuestionDimensions `json:"dimensions"`
		} `json:"questions"`
		Report string `json:"report"`
	}
	if err := json.Unmarshal([]byte(jsonText), &payload); err != nil {
		return nil, nil, "", err
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
			ID:             id,
			Label:          label,
			Title:          strings.TrimSpace(question.Title),
			SourcePages:    question.SourcePages,
			AnswerMarkdown: strings.TrimSpace(question.AnswerMarkdown),
			Status:         "detected",
			Dimensions:     &question.Dimensions,
		})
	}

	return pages, questions, strings.TrimSpace(payload.Report), nil
}
