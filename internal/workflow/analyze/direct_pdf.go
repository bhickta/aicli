package analyze

import (
	"context"
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

	geminiLiteDirectPDFMaxTokens = 24576
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

	model := firstNonBlank(req.OCRModel, req.Model)
	s.logInfo("direct PDF one-shot extraction started", "path", req.Path, "model", model, "bytes", len(data))
	res, err := processor.Document(ctx, provider.DocumentRequest{
		Model:       model,
		Prompt:      oneShotPDFPrompt(filepath.Base(req.Path)),
		Data:        data,
		MIMEType:    "application/pdf",
		Temperature: 0,
		MaxTokens:   geminiLiteDirectPDFMaxTokens,
	})
	if err != nil {
		return Response{}, err
	}
	if err := validateDirectPDFFinishReason(res.FinishReason); err != nil {
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

func validateDirectPDFFinishReason(reason string) error {
	reason = strings.ToUpper(strings.TrimSpace(reason))
	switch reason {
	case "", "STOP":
		return nil
	case "MAX_TOKENS":
		return errors.New("direct PDF response hit the output token limit; split the PDF or reduce requested detail")
	default:
		return fmt.Errorf("direct PDF response stopped with finish reason %q", reason)
	}
}
