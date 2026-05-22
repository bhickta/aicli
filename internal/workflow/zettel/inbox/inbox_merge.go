package inbox

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	progressmodel "github.com/bhickta/aicli/internal/progress"
	archivepkg "github.com/bhickta/aicli/internal/workflow/zettel/archive"
	"github.com/bhickta/aicli/internal/workflow/zettel/indexer"
	"github.com/bhickta/aicli/internal/workflow/zettel/model"
	"github.com/bhickta/aicli/internal/workflow/zettel/notetext"
	"github.com/bhickta/aicli/internal/workflow/zettel/prompt"
	"github.com/bhickta/aicli/internal/workflow/zettel/vaultfs"
)

func (r Runner) InboxMerge(ctx context.Context, req InboxMergeRequest, progress ProgressFunc) (InboxMergeResponse, error) {
	options := model.NormalizeOptions(req.Options)
	v, err := vaultfs.New(options.VaultPath)
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
		if err := r.preflightInboxMerge(ctx, v, options); err != nil {
			return response, err
		}
	}

	shorthandPrompt := prompt.LoadShorthandPrompt(options)
	runCandidates := inboxRunCandidates{}
	for i, sourcePath := range sourceNotes {
		if progress != nil {
			progress(progressmodel.Units(
				fmt.Sprintf("merging inbox note %d/%d: %s", i+1, len(sourceNotes), filepath.Base(sourcePath)),
				i,
				len(sourceNotes),
				"note",
			))
		}
		result, err := r.processInboxSource(ctx, v, archive, runID, options, sourcePath, shorthandPrompt, runCandidates, progress)
		if err != nil {
			result = InboxSourceResult{SourcePath: sourcePath, Status: inboxStatusFailed, Reason: err.Error()}
			if !archive.InboxItemExists(runID, sourcePath) {
				if archiveErr := archiveFailedInboxSource(v, archive, runID, result); archiveErr != nil {
					result.Reason = fmt.Sprintf("%s; archive failed: %s", result.Reason, archiveErr.Error())
				}
			}
		}
		switch result.Status {
		case inboxStatusProcessed:
			response.Processed = append(response.Processed, result)
		case inboxStatusPartial:
			response.Pending = append(response.Pending, result)
		case inboxStatusFailed:
			response.Failed = append(response.Failed, result)
		default:
			if result.Status == "" {
				result.Status = inboxStatusPending
			}
			response.Pending = append(response.Pending, result)
		}
		rememberInboxRunCandidates(runCandidates, result)
		if progress != nil {
			progress(progressmodel.Units(
				fmt.Sprintf("finished inbox note %d/%d: %s", i+1, len(sourceNotes), filepath.Base(sourcePath)),
				i+1,
				len(sourceNotes),
				"note",
			))
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

func (r Runner) processInboxSource(ctx context.Context, v vault, archive archivepkg.Store, runID string, options Options, sourcePath string, shorthandPrompt string, runCandidates inboxRunCandidates, progress ProgressFunc) (InboxSourceResult, error) {
	reportInboxStage(progress, sourcePath, "checking exact duplicates", 0, 6)
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

	reportInboxStage(progress, sourcePath, "embedding source and finding candidates", 1, 6)
	similar, err := indexer.New(v, options, r.embeddingProvider).Similar(ctx, sourcePath, sourceContent)
	if err != nil {
		return result, err
	}
	similar = augmentInboxCandidates(ctx, v, options, sourcePath, sourceContent, similar)
	similar = augmentInboxCandidatesFromRun(sourcePath, sourceContent, similar, runCandidates)
	if options.AdoptUnmatchedInbox {
		if adoptedPath, _, err := adoptInboxDestinationPath(v, options, sourcePath); err == nil {
			similar = appendInboxCandidatePath(similar, adoptedPath)
		}
	}
	reportInboxStage(progress, sourcePath, "building final destination notes", 2, 6)
	decision, err := r.decideInboxSource(ctx, archive, runID, sourcePath, sourceContent, similar, options, shorthandPrompt)
	if err != nil {
		return result, err
	}
	decision = constrainDecisionToCandidates(decision, similar)
	decision = constrainFinalNoteRoutes(decision, sourceContent)
	claims := decision.Claims
	result.Claims = claims
	if len(claims) == 0 {
		result.Status = inboxStatusPending
		result.Reason = "no factual claims extracted"
		if _, err := archive.WriteInboxItem(runID, result, sourceContent, nil, nil); err != nil {
			return result, err
		}
		return result, nil
	}

	applied, err := materializeInboxDecision(v, options, decision, claims)
	if err != nil {
		return result, err
	}
	ledger := applied.ledger
	destinationBefore := applied.destinationBefore
	destinationAfter := applied.destinationAfter
	destinationWrites := applied.destinationWrites
	result.Ledger = ledger
	result.DestinationPaths = applied.destinationPaths
	result.Diffs = applied.destinationDiffs
	result.MergedCount, result.DedupedCount, result.PendingCount = countLedgerStatuses(ledger)
	mechanicalAdoption := false
	if result.MergedCount+result.DedupedCount == 0 {
		if options.AdoptUnmatchedInbox && !applied.rewriteAttempted {
			reportInboxStage(progress, sourcePath, "adopting unmatched source as new zettel", 3, 6)
			adoptedPath, created, err := adoptInboxDestinationPath(v, options, sourcePath)
			if err != nil {
				return result, err
			}
			ledger = adoptedInboxLedger(claims, adoptedPath)
			result.Ledger = ledger
			result.DestinationPaths = []string{adoptedPath}
			result.MergedCount, result.DedupedCount, result.PendingCount = countLedgerStatuses(ledger)
			destinationBefore[adoptedPath] = ""
			destinationAfter[adoptedPath] = notetext.EnsureTrailingNewline(sourceContent)
			destinationWrites[adoptedPath] = destinationAfter[adoptedPath]
			result.Diffs = []InboxDestinationDiff{{
				Path:    adoptedPath,
				Before:  "",
				After:   destinationAfter[adoptedPath],
				Diff:    notetext.SimpleMarkdownDiff("", destinationAfter[adoptedPath]),
				Created: created,
			}}
			mechanicalAdoption = true
		}
	}
	if result.MergedCount+result.DedupedCount == 0 {
		result.Status = inboxStatusPending
		result.Reason = firstPendingReason(ledger, "one or more claims could not be safely merged or deduped")
		if _, err := archive.WriteInboxItem(runID, result, sourceContent, destinationBefore, destinationWrites); err != nil {
			return result, err
		}
		return result, nil
	}

	validation := mechanicalInboxValidation(mechanicalAdoption)
	if decision.FinalNotes && !mechanicalAdoption {
		validation = finalNoteInboxValidation(sourceContent, applied)
	} else if !mechanicalAdoption {
		validation = decision.Validation
	}
	result.Validation = validation
	if !mergeJudgePassed(validation, options.ValidationThreshold) {
		result.Status = inboxStatusPending
		result.Reason = "validation failed: " + validation.Notes
		result.Ledger = pendingLedgerForClaims(claims, result.Reason)
		result.MergedCount, result.DedupedCount, result.PendingCount = countLedgerStatuses(result.Ledger)
		result.DestinationPaths = nil
		result.Diffs = nil
		if _, err := archive.WriteInboxItem(runID, result, sourceContent, nil, nil); err != nil {
			return result, err
		}
		return result, nil
	}

	result.Status = inboxStatusProcessed
	if result.PendingCount > 0 {
		result.Status = inboxStatusPartial
		result.Reason = "partial merge applied; unresolved claims preserved in pending folder: " + firstPendingReason(ledger, "one or more claims could not be safely merged or deduped")
	}
	reportInboxStage(progress, sourcePath, "archiving merge result", 4, 6)
	if _, err := archive.WriteInboxItem(runID, result, sourceContent, destinationBefore, destinationWrites); err != nil {
		return result, err
	}
	reportInboxStage(progress, sourcePath, "writing destination notes", 5, 6)
	if err := writeDestinationNotes(v, options, destinationWrites); err != nil {
		return result, err
	}
	moveSource := moveInboxSourceToProcessed
	if result.Status == inboxStatusPartial {
		moveSource = moveInboxSourceToPending
	}
	reportInboxStage(progress, sourcePath, "moving source note", 5, 6)
	processedPath, err := moveSource(v, options, sourcePath)
	if err != nil {
		return result, err
	}
	result.ProcessedPath = processedPath
	if err := archive.UpdateInboxItemProcessedPath(runID, sourcePath, processedPath); err != nil {
		return result, err
	}
	return result, nil
}

func archiveFailedInboxSource(v vault, store archivepkg.Store, runID string, result InboxSourceResult) error {
	sourceContent := ""
	if sourceAbs, err := v.Abs(result.SourcePath); err == nil {
		if data, err := os.ReadFile(sourceAbs); err == nil {
			sourceContent = string(data)
		}
	}
	_, err := store.WriteInboxItem(runID, result, sourceContent, nil, nil)
	return err
}

func reportInboxStage(progress ProgressFunc, sourcePath string, stage string, completed int, total int) {
	if progress == nil {
		return
	}
	progress(progressmodel.Units(fmt.Sprintf("%s: %s", stage, filepath.Base(sourcePath)), completed, total, "stage"))
}

func appendUniquePath(paths []string, path string) []string {
	for _, existing := range paths {
		if existing == path {
			return paths
		}
	}
	return append(paths, path)
}
