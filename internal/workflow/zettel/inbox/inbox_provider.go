package inbox

import (
	"context"
	"errors"
	"strings"

	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/workflow/zettel/llmjson"
)

func (r Runner) decideInboxSource(ctx context.Context, sourcePath string, sourceContent string, candidates []scoredCandidate, options Options, shorthandPrompt string) (inboxDestinationDecision, error) {
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
	res, err := r.mergeProvider.Chat(ctx, provider.ChatRequest{
		Model:       model,
		Messages:    inboxDecisionMessages(sourcePath, sourceContent, candidates, options, shorthandPrompt),
		Temperature: 0,
	})
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
