package zettel

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	progressmodel "github.com/bhickta/aicli/internal/progress"
	archivepkg "github.com/bhickta/aicli/internal/workflow/zettel/archive"
	"github.com/bhickta/aicli/internal/workflow/zettel/notetext"
	"github.com/bhickta/aicli/internal/workflow/zettel/prompt"
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
	sourceNotes, err := v.ScanInboxNotes(options)
	if err != nil {
		return InboxMergeResponse{}, err
	}
	sort.Strings(sourceNotes)
	sourceCount := len(sourceNotes)
	if options.InboxLimit > 0 && options.InboxLimit < len(sourceNotes) {
		sourceNotes = sourceNotes[:options.InboxLimit]
	}
	runID := fmt.Sprintf("zettel-inbox-%d", time.Now().UTC().UnixNano())
	archive := archivepkg.NewStore(v, options)
	archivePath, err := archive.InboxRunPath(runID)
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
	needsPreflight, err := inboxSelectionNeedsProviderPreflight(v, options, sourceNotes)
	if err != nil {
		return response, err
	}
	if needsPreflight {
		if err := s.preflightInboxMerge(ctx, v, options); err != nil {
			return response, err
		}
	}

	shorthandPrompt := prompt.LoadShorthandPrompt(options)
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
	if err := archive.FinalizeInboxRun(runID, response); err != nil {
		return response, err
	}
	return response, nil
}

func (s *Service) processInboxSource(ctx context.Context, v vault, archive archivepkg.Store, runID string, options Options, sourcePath string, shorthandPrompt string) (InboxSourceResult, error) {
	sourceAbs, err := v.Abs(sourcePath)
	if err != nil {
		return InboxSourceResult{}, err
	}
	sourceBytes, err := os.ReadFile(sourceAbs)
	if err != nil {
		return InboxSourceResult{}, fmt.Errorf("read inbox source: %w", err)
	}
	sourceContent := string(sourceBytes)
	result := InboxSourceResult{SourcePath: sourcePath}
	if destinationPath, ok, err := findExactDestinationDuplicate(v, options, sourcePath, sourceContent); err != nil {
		return result, err
	} else if ok {
		return processExactDuplicateInboxSource(v, archive, runID, options, sourcePath, sourceContent, destinationPath)
	}

	claims, err := s.extractInboxClaims(ctx, sourcePath, sourceContent, options)
	if err != nil {
		return result, err
	}
	result.Claims = claims
	if len(claims) == 0 {
		result.Status = inboxStatusPending
		result.Reason = "no factual claims extracted"
		if _, err := archive.WriteInboxItem(runID, result, sourceContent, nil, nil); err != nil {
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
			Diff:   notetext.SimpleMarkdownDiff(before, after),
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
		if _, err := archive.WriteInboxItem(runID, result, sourceContent, destinationBefore, destinationAfter); err != nil {
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
		if _, err := archive.WriteInboxItem(runID, result, sourceContent, destinationBefore, destinationAfter); err != nil {
			return result, err
		}
		return result, nil
	}

	result.Status = inboxStatusProcessed
	if _, err := archive.WriteInboxItem(runID, result, sourceContent, destinationBefore, destinationAfter); err != nil {
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
	if err := archive.UpdateInboxItemProcessedPath(runID, sourcePath, processedPath); err != nil {
		return result, err
	}
	return result, nil
}
