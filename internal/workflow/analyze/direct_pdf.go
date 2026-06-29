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
	pdfName := filepath.Base(req.Path)
	prompts := []string{oneShotPDFPrompt(pdfName), oneShotPDFRetryPrompt(pdfName)}
	var usage *provider.TokenUsage
	var lastErr error
	for attempt, prompt := range prompts {
		res, err := processor.Document(ctx, provider.DocumentRequest{
			Model:       model,
			Prompt:      prompt,
			Data:        data,
			MIMEType:    "application/pdf",
			Temperature: 0,
			MaxTokens:   geminiLiteDirectPDFMaxTokens,
		})
		usage = addTokenUsage(usage, res.Usage)
		if err != nil {
			return Response{}, err
		}
		if err := validateDirectPDFFinishReason(res.FinishReason); err != nil {
			return Response{}, err
		}
		metadata, pages, questions, report, err := parseOneShotPDFManifest(res.Content, pdfName)
		if err != nil {
			lastErr = err
			if isIncompleteDirectPDFError(err) && attempt+1 < len(prompts) {
				s.logWarn("direct PDF extraction incomplete; retrying with coverage prompt", "path", req.Path, "attempt", attempt+1, "error", err)
				continue
			}
			return Response{}, err
		}
		s.logInfo("direct PDF one-shot extraction completed", "pages", len(pages), "questions", len(questions), "report_chars", len(report), "api_calls", attempt+1)
		return Response{
			Kind:       "topper_copy_review",
			ReviewID:   reviewID,
			PDFName:    pdfName,
			SourceMode: OCRInputModePDFDirect,
			APICalls:   attempt + 1,
			Usage:      usage,
			Metadata:   metadata,
			Pages:      pages,
			Questions:  questions,
			Report:     report,
		}, nil
	}

	return Response{}, lastErr
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

func addTokenUsage(total *provider.TokenUsage, next *provider.TokenUsage) *provider.TokenUsage {
	if next == nil {
		return total
	}
	if total == nil {
		total = &provider.TokenUsage{}
	}
	total.InputTokens += next.InputTokens
	total.CachedInputTokens += next.CachedInputTokens
	total.OutputTokens += next.OutputTokens
	total.ReasoningOutputTokens += next.ReasoningOutputTokens
	total.TotalTokens += next.TotalTokens
	return total
}
