package zettel

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

func (s *Service) extractInboxClaims(ctx context.Context, sourcePath string, sourceContent string, options Options) ([]InboxClaim, error) {
	resp, err := chatJSON[inboxClaimExtraction](ctx, s.candidateProvider, options.CandidateModel, claimExtractionMessages(sourcePath, sourceContent))
	if err != nil {
		return nil, err
	}
	return normalizeClaims(resp.Claims), nil
}

func (s *Service) routeInboxClaims(ctx context.Context, sourcePath string, claims []InboxClaim, candidates []scoredCandidate, options Options) (inboxDestinationDecision, error) {
	if len(candidates) == 0 {
		return inboxDestinationDecision{}, errors.New("no destination candidates found; run the zettel index workflow first")
	}
	return chatJSON[inboxDestinationDecision](ctx, s.candidateProvider, options.CandidateModel, inboxDestinationMessages(sourcePath, claims, candidates, options))
}

func (s *Service) rewriteInboxDestination(ctx context.Context, v vault, options Options, destinationPath string, sourcePath string, claims []InboxClaim, shorthandPrompt string) (string, string, inboxRewritePlan, error) {
	destinationAbs, err := v.NotePath(destinationPath, options)
	if err != nil {
		return "", "", inboxRewritePlan{}, err
	}
	beforeBytes, err := os.ReadFile(destinationAbs)
	if err != nil {
		return "", "", inboxRewritePlan{}, fmt.Errorf("read destination note: %w", err)
	}
	before := string(beforeBytes)
	plan, err := chatJSON[inboxRewritePlan](ctx, s.mergeProvider, options.MergeModel, inboxRewriteMessages(destinationPath, before, sourcePath, claims, shorthandPrompt))
	if err != nil {
		return "", "", inboxRewritePlan{}, err
	}
	if strings.TrimSpace(plan.FinalMarkdown) == "" {
		return "", "", inboxRewritePlan{}, errors.New("rewrite returned empty final markdown")
	}
	return before, ensureTrailingNewline(plan.FinalMarkdown), plan, nil
}

func (s *Service) validateInboxMerge(ctx context.Context, sourcePath string, sourceContent string, destinationBefore map[string]string, destinationAfter map[string]string, ledger []InboxClaimLedger, options Options) (MergeJudge, error) {
	return chatJSON[MergeJudge](ctx, s.validationProvider, options.ValidationModel, inboxValidationMessages(sourcePath, sourceContent, destinationBefore, destinationAfter, ledger))
}
