package inbox

import (
	"context"
	"errors"
	"strings"

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
	resp, err := llmjson.Chat[inboxDestinationDecision](ctx, r.mergeProvider, model, inboxDecisionMessages(sourcePath, sourceContent, candidates, options, shorthandPrompt))
	if err != nil {
		return inboxDestinationDecision{}, err
	}
	resp.Claims = normalizeClaims(resp.Claims)
	return resp, nil
}
