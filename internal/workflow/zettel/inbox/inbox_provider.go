package inbox

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/bhickta/aicli/internal/workflow/zettel/llmjson"
	"github.com/bhickta/aicli/internal/workflow/zettel/notetext"
)

func (r Runner) extractInboxClaims(ctx context.Context, sourcePath string, sourceContent string, options Options) ([]InboxClaim, error) {
	resp, err := llmjson.Chat[inboxClaimExtraction](ctx, r.candidateProvider, options.CandidateModel, claimExtractionMessages(sourcePath, sourceContent))
	if err != nil {
		return nil, err
	}
	return normalizeClaims(resp.Claims), nil
}

func (r Runner) routeInboxClaims(ctx context.Context, sourcePath string, claims []InboxClaim, candidates []scoredCandidate, options Options) (inboxDestinationDecision, error) {
	if len(candidates) == 0 {
		return inboxDestinationDecision{}, errors.New("no destination candidates found; run the zettel index workflow first")
	}
	return llmjson.Chat[inboxDestinationDecision](ctx, r.candidateProvider, options.CandidateModel, inboxDestinationMessages(sourcePath, claims, candidates, options))
}

func (r Runner) rewriteInboxDestination(ctx context.Context, v vault, options Options, destinationPath string, sourcePath string, claims []InboxClaim, shorthandPrompt string) (string, string, inboxRewritePlan, error) {
	destinationAbs, err := v.NotePath(destinationPath, options)
	if err != nil {
		return "", "", inboxRewritePlan{}, err
	}
	beforeBytes, err := os.ReadFile(destinationAbs)
	if err != nil {
		return "", "", inboxRewritePlan{}, fmt.Errorf("read destination note: %w", err)
	}
	before := string(beforeBytes)
	plan, err := llmjson.Chat[inboxRewritePlan](ctx, r.mergeProvider, options.MergeModel, inboxRewriteMessages(destinationPath, before, sourcePath, claims, shorthandPrompt))
	if err != nil {
		return "", "", inboxRewritePlan{}, err
	}
	if strings.TrimSpace(plan.FinalMarkdown) == "" {
		return "", "", inboxRewritePlan{}, errors.New("rewrite returned empty final markdown")
	}
	return before, notetext.EnsureTrailingNewline(plan.FinalMarkdown), plan, nil
}

func (r Runner) validateInboxMerge(ctx context.Context, sourcePath string, sourceContent string, destinationBefore map[string]string, destinationAfter map[string]string, ledger []InboxClaimLedger, options Options) (MergeJudge, error) {
	return llmjson.Chat[MergeJudge](ctx, r.validationProvider, options.ValidationModel, inboxValidationMessages(sourcePath, sourceContent, destinationBefore, destinationAfter, ledger))
}
