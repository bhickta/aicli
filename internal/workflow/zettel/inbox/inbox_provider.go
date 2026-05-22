package inbox

import (
	"context"
	"errors"
	"strings"

	"github.com/bhickta/aicli/internal/provider"
	archivepkg "github.com/bhickta/aicli/internal/workflow/zettel/archive"
	"github.com/bhickta/aicli/internal/workflow/zettel/llmjson"
)

func (r Runner) decideInboxSource(
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
		return inboxDestinationDecision{}, errors.New("no destination candidates found; run the zettel index workflow first")
	}
	model := strings.TrimSpace(options.MergeModel)
	if model == "" {
		model = options.CandidateModel
	}
	if r.mergeProvider == nil {
		return inboxDestinationDecision{}, errors.New("provider is required")
	}
	messages := inboxDecisionMessages(sourcePath, sourceContent, candidates, options, shorthandPrompt)
	req := provider.ChatRequest{
		Model:       model,
		Messages:    messages,
		Temperature: 0,
	}
	res, err := r.mergeProvider.Chat(ctx, req)
	parsedFormat := "unparsed"
	if err == nil {
		if _, ok := parseInboxFinalNotes(sourcePath, res.Content); ok {
			parsedFormat = "final-notes"
		} else {
			parsedFormat = "json"
		}
	}
	if _, traceErr := archive.WriteInboxLLMExchange(runID, archivepkg.LLMExchange{
		Step:         "build-final-destination-notes",
		SourcePath:   sourcePath,
		ProviderID:   inboxProviderID(r.mergeProvider),
		Model:        model,
		Request:      req,
		Response:     res,
		Error:        errorString(err),
		ParsedFormat: parsedFormat,
	}); traceErr != nil {
		if err != nil {
			return inboxDestinationDecision{}, errors.Join(err, traceErr)
		}
		return inboxDestinationDecision{}, traceErr
	}
	if err != nil {
		return inboxDestinationDecision{}, err
	}
	if decision, ok := parseInboxFinalNotes(sourcePath, res.Content); ok {
		return decision, nil
	}
	resp, err := llmjson.Parse[inboxDestinationDecision](res.Content)
	if err != nil {
		return inboxDestinationDecision{}, err
	}
	resp.Claims = normalizeClaims(resp.Claims)
	return resp, nil
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
