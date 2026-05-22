package inbox

import (
	"context"
	"errors"
	"strings"

	"github.com/bhickta/aicli/internal/provider"
	archivepkg "github.com/bhickta/aicli/internal/workflow/zettel/archive"
)

func (r Runner) judgeInboxDestinations(
	ctx context.Context,
	archive archivepkg.Store,
	runID string,
	sourcePath string,
	sourceContent string,
	candidates []scoredCandidate,
	options Options,
	shorthandPrompt string,
) (inboxDestinationJudgement, error) {
	if len(candidates) == 0 {
		return inboxDestinationJudgement{}, errors.New("no destination candidates found; run the zettel index workflow first")
	}
	if r.mergeProvider == nil {
		return inboxDestinationJudgement{}, errors.New("provider is required")
	}
	model := strings.TrimSpace(options.MergeModel)
	req := provider.ChatRequest{
		Model:       model,
		Messages:    inboxJudgeMessages(sourcePath, sourceContent, candidates, options, shorthandPrompt),
		Temperature: 0,
	}
	res, err := r.mergeProvider.Chat(ctx, req)
	parsedFormat := "unparsed"
	var judgement inboxDestinationJudgement
	if err == nil {
		var ok bool
		judgement, ok = parseInboxDestinationJudgement(res.Content)
		if ok {
			if judgement.PendingReason != "" {
				parsedFormat = "pending"
			} else {
				parsedFormat = "targets"
			}
		} else {
			parsedFormat = "invalid-targets"
		}
	}
	if traceErr := writeInboxLLMExchange(archive, runID, "judge-destinations", sourcePath, r.mergeProvider, model, req, res, err, parsedFormat); traceErr != nil {
		if err != nil {
			return inboxDestinationJudgement{}, errors.Join(err, traceErr)
		}
		return inboxDestinationJudgement{}, traceErr
	}
	if err != nil {
		return inboxDestinationJudgement{}, err
	}
	if judgement.PendingReason != "" || len(judgement.Targets) > 0 {
		return judgement, nil
	}
	return inboxDestinationJudgement{}, errors.New("judge model response must use TARGET lines or PENDING")
}

func (r Runner) buildInboxFinalNotes(
	ctx context.Context,
	archive archivepkg.Store,
	runID string,
	sourcePath string,
	sourceContent string,
	candidates []scoredCandidate,
	options Options,
	shorthandPrompt string,
) (inboxDestinationDecision, error) {
	if len(candidates) == 0 {
		return inboxDestinationDecision{}, errors.New("no approved destination candidates found")
	}
	if r.mergeProvider == nil {
		return inboxDestinationDecision{}, errors.New("provider is required")
	}
	model := strings.TrimSpace(options.MergeModel)
	req := provider.ChatRequest{
		Model:       model,
		Messages:    inboxMergeMessages(sourcePath, sourceContent, candidates, options, shorthandPrompt),
		Temperature: 0,
	}
	res, err := r.mergeProvider.Chat(ctx, req)
	parsedFormat := "unparsed"
	var decision inboxDestinationDecision
	if err == nil {
		var ok bool
		decision, ok = parseInboxFinalNotes(sourcePath, res.Content)
		if ok {
			if len(decision.Destinations) == 0 {
				parsedFormat = "pending"
			} else {
				parsedFormat = "final-notes"
			}
		} else {
			parsedFormat = "invalid-final-notes"
		}
	}
	if traceErr := writeInboxLLMExchange(archive, runID, "merge-final-notes", sourcePath, r.mergeProvider, model, req, res, err, parsedFormat); traceErr != nil {
		if err != nil {
			return inboxDestinationDecision{}, errors.Join(err, traceErr)
		}
		return inboxDestinationDecision{}, traceErr
	}
	if err != nil {
		return inboxDestinationDecision{}, err
	}
	if len(decision.Destinations) > 0 || len(decision.Pending) > 0 {
		return decision, nil
	}
	return inboxDestinationDecision{}, errors.New("merge model response must use BEGIN_NOTE/END_NOTE blocks or PENDING")
}

func (r Runner) validateInboxFinalNotes(
	ctx context.Context,
	archive archivepkg.Store,
	runID string,
	sourcePath string,
	sourceContent string,
	candidates []scoredCandidate,
	decision inboxDestinationDecision,
	options Options,
) error {
	if len(decision.Destinations) == 0 {
		return nil
	}
	if r.mergeProvider == nil {
		return errors.New("provider is required")
	}
	model := strings.TrimSpace(options.MergeModel)
	req := provider.ChatRequest{
		Model:       model,
		Messages:    inboxValidationMessages(sourcePath, sourceContent, candidates, decision),
		Temperature: 0,
	}
	res, err := r.mergeProvider.Chat(ctx, req)
	parsedFormat := "unparsed"
	var validation inboxValidationResult
	if err == nil {
		validation = parseInboxValidationResult(res.Content)
		if validation.OK {
			if validation.Pass {
				parsedFormat = "pass"
			} else {
				parsedFormat = "fail"
			}
		} else {
			parsedFormat = "invalid-validation"
		}
	}
	if traceErr := writeInboxLLMExchange(archive, runID, "validate-final-notes", sourcePath, r.mergeProvider, model, req, res, err, parsedFormat); traceErr != nil {
		if err != nil {
			return errors.Join(err, traceErr)
		}
		return traceErr
	}
	if err != nil {
		return err
	}
	if !validation.OK {
		return errors.New("validator response must be PASS or FAIL")
	}
	if !validation.Pass {
		return errors.New("validation failed: " + validation.Reason)
	}
	return nil
}

func writeInboxLLMExchange(
	archive archivepkg.Store,
	runID string,
	step string,
	sourcePath string,
	p provider.Provider,
	model string,
	req provider.ChatRequest,
	res provider.ChatResponse,
	err error,
	parsedFormat string,
) error {
	_, traceErr := archive.WriteInboxLLMExchange(runID, archivepkg.LLMExchange{
		Step:         step,
		SourcePath:   sourcePath,
		ProviderID:   inboxProviderID(p),
		Model:        model,
		Request:      req,
		Response:     res,
		Error:        errorString(err),
		ParsedFormat: parsedFormat,
	})
	return traceErr
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func inboxProviderID(p provider.Provider) string {
	if p == nil {
		return ""
	}
	return p.ID()
}
