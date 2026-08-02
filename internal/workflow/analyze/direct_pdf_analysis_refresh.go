package analyze

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/workflow/document"
)

const maxDirectPDFAnalysisRefreshAttempts = 2

func (s *Service) refreshDirectPDFInternalProcessingAnalysis(
	ctx context.Context,
	req Request,
	model string,
	tempDir string,
	questions []Question,
	processor provider.DocumentProcessor,
) ([]Question, *provider.TokenUsage, int, error) {
	refreshed := append([]Question(nil), questions...)
	var usage *provider.TokenUsage
	calls := 0
	for index := range refreshed {
		question := refreshed[index]
		analysisErr := validateDirectPDFQuestionAnalysisLanguage([]Question{question})
		if analysisErr == nil {
			continue
		}
		s.logWarn(
			"direct PDF question analysis uses internal processing explanation; refreshing from evidence",
			"question", firstNonBlank(question.Label, question.ID),
			"error", analysisErr,
		)
		if len(question.SourcePages) == 0 {
			return nil, usage, calls, fmt.Errorf("refresh direct PDF analysis for question %q: no source pages", question.ID)
		}
		firstPage, lastPage := question.SourcePages[0], question.SourcePages[0]
		for _, page := range question.SourcePages[1:] {
			firstPage = min(firstPage, page)
			lastPage = max(lastPage, page)
		}
		questionPDF := filepath.Join(tempDir, fmt.Sprintf("analysis-refresh-%03d-p%04d-p%04d.pdf", index+1, firstPage, lastPage))
		if err := document.SplitPDFRange(ctx, s.runner, s.tools.QPDF, req.Path, questionPDF, firstPage, lastPage); err != nil {
			return nil, usage, calls, fmt.Errorf("prepare direct PDF analysis refresh for question %q: %w", question.ID, err)
		}
		data, err := os.ReadFile(questionPDF)
		if err != nil {
			return nil, usage, calls, fmt.Errorf("read direct PDF analysis refresh for question %q: %w", question.ID, err)
		}

		var previousResponse string
		var lastErr error
		updated := false
		for attempt := 0; attempt < maxDirectPDFAnalysisRefreshAttempts; attempt++ {
			res, err := processor.Document(ctx, provider.DocumentRequest{
				Model:            model,
				Prompt:           directPDFAnalysisRefreshPrompt(question, firstPage, lastPage, previousResponse, lastErr),
				Data:             data,
				MIMEType:         "application/pdf",
				ResponseMIMEType: "application/json",
				ResponseSchema:   questionDimensionsJSONSchema(),
				Temperature:      0,
				MaxTokens:        3200,
			})
			usage = addTokenUsage(usage, res.Usage)
			calls++
			if err != nil {
				return nil, usage, calls, fmt.Errorf("refresh direct PDF analysis for question %q: %w", question.ID, err)
			}
			if err := validateDirectPDFFinishReason(res.FinishReason); err != nil {
				lastErr = err
				previousResponse = res.Content
				continue
			}
			dimensions, err := parseQuestionDimensions(res.Content)
			if err == nil {
				candidate := question
				candidate.Dimensions = dimensions
				err = validateDirectPDFQuestionAnalysisLanguage([]Question{candidate})
				if err == nil {
					refreshed[index].Dimensions = dimensions
					updated = true
					break
				}
			}
			lastErr = err
			previousResponse = res.Content
		}
		if !updated {
			return nil, usage, calls, fmt.Errorf("refresh direct PDF analysis for question %q after %d attempts: %w", question.ID, maxDirectPDFAnalysisRefreshAttempts, lastErr)
		}
		s.logInfo(
			"direct PDF question analysis refreshed from evidence",
			"question", firstNonBlank(question.Label, question.ID),
			"first_page", firstPage,
			"last_page", lastPage,
		)
	}
	if err := validateDirectPDFQuestionAnalysisLanguage(refreshed); err != nil {
		return nil, usage, calls, err
	}
	return refreshed, usage, calls, nil
}

func directPDFAnalysisRefreshPrompt(
	question Question,
	firstPage int,
	lastPage int,
	previousResponse string,
	previousErr error,
) string {
	prefix := ""
	if previousErr != nil {
		prefix = fmt.Sprintf(`Your previous analysis was rejected.
Validator violation: %s
Rejected response:
<rejected_response>
%s
</rejected_response>

Return a corrected complete JSON object, not a patch.

`, strings.TrimSpace(previousErr.Error()), strings.TrimSpace(previousResponse))
	}
	return prefix + fmt.Sprintf(`The attached PDF contains original global pages %d-%d for one reconciled UPSC/Mains answer.
Replace the complete analytical dimensions for this answer using only visible evidence in the attachment and the answer OCR below.
Internal extraction, overlap, and processing boundaries are not candidate-answer evidence. Never attribute incompleteness, truncation, weakness, or uncertainty to them. If writing visibly ends, state exactly where it ends; if an attached answer area is blank, say that it is visibly blank.

%s

For handwriting, layout, diagrams, evaluator marks, and visible stopping points, inspect the attachment directly; it is authoritative over the OCR representation.`, firstPage, lastPage, questionDimensionsPrompt(question))
}
