package zettel

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	progressmodel "github.com/bhickta/aicli/internal/progress"
)

const (
	inboxStatusProcessed = "processed"
	inboxStatusPending   = "pending"
	inboxStatusFailed    = "failed"
	claimStatusMerged    = "merged"
	claimStatusDeduped   = "deduped"
	claimStatusPending   = "pending"
)

func (s *Service) InboxMerge(ctx context.Context, req InboxMergeRequest, progress ProgressFunc) (InboxMergeResponse, error) {
	options := normalizeOptions(req.Options)
	v, err := newVault(options.VaultPath)
	if err != nil {
		return InboxMergeResponse{}, err
	}
	sourceNotes, err := v.scanInboxNotes(options)
	if err != nil {
		return InboxMergeResponse{}, err
	}
	sort.Strings(sourceNotes)
	sourceCount := len(sourceNotes)
	if options.InboxLimit > 0 && options.InboxLimit < len(sourceNotes) {
		sourceNotes = sourceNotes[:options.InboxLimit]
	}
	runID := fmt.Sprintf("zettel-inbox-%d", time.Now().UTC().UnixNano())
	archive := newArchiveStore(v, options)
	archivePath, err := archive.inboxRunPath(runID)
	if err != nil {
		return InboxMergeResponse{}, err
	}
	response := InboxMergeResponse{
		RunID:         runID,
		ArchivePath:   archivePath,
		SourceCount:   sourceCount,
		SelectedCount: len(sourceNotes),
		SkippedCount:  sourceCount - len(sourceNotes),
		Limit:         options.InboxLimit,
	}
	if len(sourceNotes) == 0 {
		return response, nil
	}
	if err := s.preflightInboxMerge(ctx, v, options); err != nil {
		return response, err
	}

	shorthandPrompt := loadShorthandPrompt(options)
	for i, sourcePath := range sourceNotes {
		if progress != nil {
			progress(progressmodel.Units(
				fmt.Sprintf("merging inbox note %d/%d: %s", i+1, len(sourceNotes), filepath.Base(sourcePath)),
				i,
				len(sourceNotes),
				"note",
			))
		}
		result, err := s.processInboxSource(ctx, v, archive, runID, options, sourcePath, shorthandPrompt)
		if err != nil {
			result = InboxSourceResult{SourcePath: sourcePath, Status: inboxStatusFailed, Reason: err.Error()}
		}
		switch result.Status {
		case inboxStatusProcessed:
			response.Processed = append(response.Processed, result)
		case inboxStatusFailed:
			response.Failed = append(response.Failed, result)
		default:
			if result.Status == "" {
				result.Status = inboxStatusPending
			}
			response.Pending = append(response.Pending, result)
		}
	}
	if progress != nil {
		progress(progressmodel.Units("completed inbox merge run", len(sourceNotes), len(sourceNotes), "note"))
	}
	response.ProcessedCount = len(response.Processed)
	response.PendingCount = len(response.Pending)
	response.FailedCount = len(response.Failed)
	return response, nil
}

func (s *Service) preflightInboxMerge(ctx context.Context, v vault, options Options) error {
	index := newEmbeddingIndex(v, options, s.embeddingProvider)
	cache, err := index.load()
	if err != nil {
		return fmt.Errorf("load zettelkasten embedding index: %w", err)
	}
	if len(cache.Items) == 0 {
		return errors.New("zettelkasten embedding index is empty; run Build Index after selecting the destination notes folder")
	}
	if _, err := index.embed(ctx, []string{"zettelkasten inbox merge embedding preflight"}); err != nil {
		return fmt.Errorf("embedding provider unavailable for inbox merge: %w", err)
	}
	return nil
}

func (s *Service) processInboxSource(ctx context.Context, v vault, archive archiveStore, runID string, options Options, sourcePath string, shorthandPrompt string) (InboxSourceResult, error) {
	sourceAbs, err := v.abs(sourcePath)
	if err != nil {
		return InboxSourceResult{}, err
	}
	sourceBytes, err := os.ReadFile(sourceAbs)
	if err != nil {
		return InboxSourceResult{}, fmt.Errorf("read inbox source: %w", err)
	}
	sourceContent := string(sourceBytes)
	result := InboxSourceResult{SourcePath: sourcePath}

	claims, err := s.extractInboxClaims(ctx, sourcePath, sourceContent, options)
	if err != nil {
		return result, err
	}
	result.Claims = claims
	if len(claims) == 0 {
		result.Status = inboxStatusPending
		result.Reason = "no factual claims extracted"
		if _, err := archive.writeInboxItem(runID, result, sourceContent, nil, nil); err != nil {
			return result, err
		}
		return result, nil
	}

	similar, err := newEmbeddingIndex(v, options, s.embeddingProvider).Similar(ctx, sourcePath, sourceContent)
	if err != nil {
		return result, err
	}
	decision, err := s.routeInboxClaims(ctx, sourcePath, claims, similar, options)
	if err != nil {
		return result, err
	}

	assignments, ledger := normalizeInboxAssignments(decision, claims, options)
	destinationBefore := map[string]string{}
	destinationAfter := map[string]string{}
	destinationDiffs := []InboxDestinationDiff{}
	destinationPaths := make([]string, 0, len(assignments))
	for destinationPath, claimIDs := range assignments {
		assignedClaims := selectClaims(claims, claimIDs)
		if len(assignedClaims) == 0 {
			continue
		}
		before, after, plan, err := s.rewriteInboxDestination(ctx, v, options, destinationPath, sourcePath, assignedClaims, shorthandPrompt)
		if err != nil {
			ledger = append(ledger, pendingLedgerForClaims(assignedClaims, fmt.Sprintf("rewrite failed: %s", err.Error()))...)
			continue
		}
		destinationBefore[destinationPath] = before
		destinationAfter[destinationPath] = after
		destinationPaths = append(destinationPaths, destinationPath)
		ledger = append(ledger, normalizeRewriteLedger(plan.Ledger, destinationPath, assignedClaims)...)
		destinationDiffs = append(destinationDiffs, InboxDestinationDiff{
			Path:   destinationPath,
			Before: before,
			After:  after,
			Diff:   simpleMarkdownDiff(before, after),
		})
	}

	ledger = ensureAllClaimsAccounted(claims, ledger)
	result.Ledger = ledger
	result.DestinationPaths = destinationPaths
	result.Diffs = destinationDiffs
	result.MergedCount, result.DedupedCount, result.PendingCount = countLedgerStatuses(ledger)
	if result.PendingCount > 0 || len(destinationAfter) == 0 {
		result.Status = inboxStatusPending
		result.Reason = firstPendingReason(ledger, "one or more claims could not be safely merged or deduped")
		if _, err := archive.writeInboxItem(runID, result, sourceContent, destinationBefore, destinationAfter); err != nil {
			return result, err
		}
		return result, nil
	}

	validation, err := s.validateInboxMerge(ctx, sourcePath, sourceContent, destinationBefore, destinationAfter, ledger, options)
	if err != nil {
		return result, err
	}
	result.Validation = validation
	if !mergeJudgePassed(validation, options.ValidationThreshold) {
		result.Status = inboxStatusPending
		result.Reason = "validation failed: " + validation.Notes
		if _, err := archive.writeInboxItem(runID, result, sourceContent, destinationBefore, destinationAfter); err != nil {
			return result, err
		}
		return result, nil
	}

	result.Status = inboxStatusProcessed
	if _, err := archive.writeInboxItem(runID, result, sourceContent, destinationBefore, destinationAfter); err != nil {
		return result, err
	}
	if err := writeDestinationNotes(v, options, destinationAfter); err != nil {
		return result, err
	}
	processedPath, err := moveInboxSourceToProcessed(v, options, sourcePath)
	if err != nil {
		return result, err
	}
	result.ProcessedPath = processedPath
	if err := archive.updateInboxItemProcessedPath(runID, sourcePath, processedPath); err != nil {
		return result, err
	}
	return result, nil
}

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
	destinationAbs, err := v.notePath(destinationPath, options)
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

func normalizeClaims(claims []InboxClaim) []InboxClaim {
	out := make([]InboxClaim, 0, len(claims))
	seen := map[string]int{}
	for i, claim := range claims {
		claim.Text = strings.TrimSpace(claim.Text)
		if claim.Text == "" {
			continue
		}
		claim.ID = strings.TrimSpace(claim.ID)
		if claim.ID == "" {
			claim.ID = fmt.Sprintf("c%d", i+1)
		}
		if count := seen[claim.ID]; count > 0 {
			claim.ID = fmt.Sprintf("%s-%d", claim.ID, count+1)
		}
		seen[claim.ID]++
		out = append(out, claim)
	}
	return out
}

func normalizeInboxAssignments(decision inboxDestinationDecision, claims []InboxClaim, options Options) (map[string][]string, []InboxClaimLedger) {
	claimSet := claimIDSet(claims)
	assignments := map[string][]string{}
	assigned := map[string]bool{}
	ledger := []InboxClaimLedger{}
	for _, pending := range decision.Pending {
		if claimSet[pending.ClaimID] {
			pending.Status = claimStatusPending
			ledger = append(ledger, pending)
		}
	}
	for _, destination := range decision.Destinations {
		path := strings.TrimSpace(destination.Path)
		if path == "" || destination.Confidence < options.ReviewThreshold {
			for _, id := range destination.ClaimIDs {
				if claimSet[id] {
					ledger = append(ledger, InboxClaimLedger{ClaimID: id, Status: claimStatusPending, Reason: "destination confidence below threshold"})
				}
			}
			continue
		}
		for _, id := range destination.ClaimIDs {
			if !claimSet[id] || assigned[id] {
				continue
			}
			assignments[path] = append(assignments[path], id)
			assigned[id] = true
		}
	}
	return assignments, ledger
}

func claimIDSet(claims []InboxClaim) map[string]bool {
	out := make(map[string]bool, len(claims))
	for _, claim := range claims {
		out[claim.ID] = true
	}
	return out
}

func selectClaims(claims []InboxClaim, ids []string) []InboxClaim {
	allowed := map[string]bool{}
	for _, id := range ids {
		allowed[id] = true
	}
	out := make([]InboxClaim, 0, len(ids))
	for _, claim := range claims {
		if allowed[claim.ID] {
			out = append(out, claim)
		}
	}
	return out
}

func normalizeRewriteLedger(ledger []InboxClaimLedger, destinationPath string, claims []InboxClaim) []InboxClaimLedger {
	claimSet := claimIDSet(claims)
	out := make([]InboxClaimLedger, 0, len(ledger))
	for _, item := range ledger {
		if !claimSet[item.ClaimID] {
			continue
		}
		item.Status = strings.ToLower(strings.TrimSpace(item.Status))
		if item.Status != claimStatusMerged && item.Status != claimStatusDeduped && item.Status != claimStatusPending {
			item.Status = claimStatusPending
			if item.Reason == "" {
				item.Reason = "unknown ledger status"
			}
		}
		if item.DestinationPath == "" {
			item.DestinationPath = destinationPath
		}
		out = append(out, item)
	}
	return out
}

func pendingLedgerForClaims(claims []InboxClaim, reason string) []InboxClaimLedger {
	out := make([]InboxClaimLedger, 0, len(claims))
	for _, claim := range claims {
		out = append(out, InboxClaimLedger{ClaimID: claim.ID, Status: claimStatusPending, Reason: reason})
	}
	return out
}

func ensureAllClaimsAccounted(claims []InboxClaim, ledger []InboxClaimLedger) []InboxClaimLedger {
	accounted := map[string]bool{}
	for _, item := range ledger {
		if item.Status == claimStatusMerged || item.Status == claimStatusDeduped {
			accounted[item.ClaimID] = true
		}
	}
	seen := map[string]bool{}
	out := make([]InboxClaimLedger, 0, len(ledger)+len(claims))
	for _, item := range ledger {
		if item.Status == claimStatusPending && accounted[item.ClaimID] {
			continue
		}
		key := item.ClaimID + "\x00" + item.Status + "\x00" + item.DestinationPath
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	for _, claim := range claims {
		if !accounted[claim.ID] {
			out = append(out, InboxClaimLedger{ClaimID: claim.ID, Status: claimStatusPending, Reason: "claim was not accounted for"})
		}
	}
	return out
}

func countLedgerStatuses(ledger []InboxClaimLedger) (int, int, int) {
	merged := 0
	deduped := 0
	pending := 0
	seenAccounted := map[string]bool{}
	for _, item := range ledger {
		switch item.Status {
		case claimStatusMerged:
			if !seenAccounted[item.ClaimID] {
				merged++
				seenAccounted[item.ClaimID] = true
			}
		case claimStatusDeduped:
			if !seenAccounted[item.ClaimID] {
				deduped++
				seenAccounted[item.ClaimID] = true
			}
		case claimStatusPending:
			pending++
		}
	}
	return merged, deduped, pending
}

func firstPendingReason(ledger []InboxClaimLedger, fallback string) string {
	for _, item := range ledger {
		if item.Status == claimStatusPending && strings.TrimSpace(item.Reason) != "" {
			return item.Reason
		}
	}
	return fallback
}

func mergeJudgePassed(judge MergeJudge, threshold float64) bool {
	return strings.EqualFold(judge.Verdict, "pass") && judge.Score >= threshold
}

func writeDestinationNotes(v vault, options Options, destinationAfter map[string]string) error {
	paths := make([]string, 0, len(destinationAfter))
	for path := range destinationAfter {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		abs, err := v.notePath(path, options)
		if err != nil {
			return err
		}
		if err := os.WriteFile(abs, []byte(ensureTrailingNewline(destinationAfter[path])), 0o600); err != nil {
			return fmt.Errorf("write destination note %s: %w", path, err)
		}
	}
	return nil
}

func moveInboxSourceToProcessed(v vault, options Options, sourcePath string) (string, error) {
	inbox := strings.Trim(filepath.ToSlash(filepath.Clean(options.InboxFolder)), "/")
	source := strings.Trim(filepath.ToSlash(filepath.Clean(sourcePath)), "/")
	relInside := strings.TrimPrefix(source, inbox)
	relInside = strings.Trim(relInside, "/")
	if relInside == "" {
		relInside = filepath.Base(source)
	}
	processedRel := filepath.ToSlash(filepath.Join(inbox, "_processed", time.Now().Format("2006-01-02"), relInside))
	processedAbs, err := v.abs(processedRel)
	if err != nil {
		return "", err
	}
	processedAbs = uniquePath(processedAbs)
	sourceAbs, err := v.abs(sourcePath)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(processedAbs), 0o755); err != nil {
		return "", fmt.Errorf("create processed folder: %w", err)
	}
	if err := os.Rename(sourceAbs, processedAbs); err != nil {
		return "", fmt.Errorf("move processed source: %w", err)
	}
	rel, err := v.rel(processedAbs)
	if err != nil {
		return "", err
	}
	return rel, nil
}

func uniquePath(path string) string {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return path
	}
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d%s", base, i, ext)
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate
		}
	}
}
