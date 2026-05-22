package inbox

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	progressmodel "github.com/bhickta/aicli/internal/progress"
	archivepkg "github.com/bhickta/aicli/internal/workflow/zettel/archive"
	"github.com/bhickta/aicli/internal/workflow/zettel/indexer"
	"github.com/bhickta/aicli/internal/workflow/zettel/model"
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
	if options.InboxRandom {
		shuffleSourceNotes(sourceNotes)
	}
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
	results := r.processInboxSources(ctx, v, archive, runID, options, sourceNotes, shorthandPrompt, progress)
	for _, result := range results {
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

type inboxRunLocks struct {
	archive  sync.Mutex
	commit   sync.Mutex
	progress sync.Mutex
}

type inboxSourceJob struct {
	index int
	path  string
}

type inboxSourceOutcome struct {
	index  int
	result InboxSourceResult
}

func (r Runner) processInboxSources(
	ctx context.Context,
	v vault,
	archive archivepkg.Store,
	runID string,
	options Options,
	sourceNotes []string,
	shorthandPrompt string,
	progress ProgressFunc,
) []InboxSourceResult {
	workers := normalizedInboxWorkers(options.InboxWorkers, len(sourceNotes))
	locks := &inboxRunLocks{}
	progress = synchronizedInboxProgress(progress, locks)
	if workers <= 1 {
		return r.processInboxSourcesSequential(ctx, v, archive, runID, options, sourceNotes, shorthandPrompt, progress, locks)
	}
	return r.processInboxSourcesParallel(ctx, v, archive, runID, options, sourceNotes, shorthandPrompt, progress, locks, workers)
}

func (r Runner) processInboxSourcesSequential(
	ctx context.Context,
	v vault,
	archive archivepkg.Store,
	runID string,
	options Options,
	sourceNotes []string,
	shorthandPrompt string,
	progress ProgressFunc,
	locks *inboxRunLocks,
) []InboxSourceResult {
	results := make([]InboxSourceResult, 0, len(sourceNotes))
	for i, sourcePath := range sourceNotes {
		reportInboxNoteProgress(progress, "merging", i, len(sourceNotes), sourcePath)
		result := r.processInboxSourceJob(ctx, v, archive, runID, options, sourcePath, shorthandPrompt, progress, locks)
		results = append(results, result)
		reportInboxNoteProgress(progress, "finished", i+1, len(sourceNotes), sourcePath)
	}
	return results
}

func (r Runner) processInboxSourcesParallel(
	ctx context.Context,
	v vault,
	archive archivepkg.Store,
	runID string,
	options Options,
	sourceNotes []string,
	shorthandPrompt string,
	progress ProgressFunc,
	locks *inboxRunLocks,
	workers int,
) []InboxSourceResult {
	jobs := make(chan inboxSourceJob)
	outcomes := make(chan inboxSourceOutcome, len(sourceNotes))
	var completed atomic.Int64
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				reportInboxNoteProgress(progress, "merging", int(completed.Load()), len(sourceNotes), job.path)
				result := r.processInboxSourceJob(ctx, v, archive, runID, options, job.path, shorthandPrompt, progress, locks)
				done := int(completed.Add(1))
				reportInboxNoteProgress(progress, "finished", done, len(sourceNotes), job.path)
				outcomes <- inboxSourceOutcome{index: job.index, result: result}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for i, sourcePath := range sourceNotes {
			select {
			case <-ctx.Done():
				return
			case jobs <- inboxSourceJob{index: i, path: sourcePath}:
			}
		}
	}()
	go func() {
		wg.Wait()
		close(outcomes)
	}()

	results := make([]InboxSourceResult, len(sourceNotes))
	for outcome := range outcomes {
		results[outcome.index] = outcome.result
	}
	for i, result := range results {
		if result.SourcePath == "" {
			sourcePath := sourceNotes[i]
			reason := "inbox merge stopped before this note completed"
			if err := ctx.Err(); err != nil {
				reason = err.Error()
			}
			results[i] = InboxSourceResult{
				SourcePath: sourcePath,
				Status:     inboxStatusFailed,
				Reason:     reason,
			}
		}
	}
	return results
}

func (r Runner) processInboxSourceJob(
	ctx context.Context,
	v vault,
	archive archivepkg.Store,
	runID string,
	options Options,
	sourcePath string,
	shorthandPrompt string,
	progress ProgressFunc,
	locks *inboxRunLocks,
) InboxSourceResult {
	result, err := r.processInboxSource(ctx, v, archive, runID, options, sourcePath, shorthandPrompt, progress, locks)
	if err == nil {
		return result
	}
	result = InboxSourceResult{SourcePath: sourcePath, Status: inboxStatusFailed, Reason: err.Error()}
	locks.archive.Lock()
	defer locks.archive.Unlock()
	if !archive.InboxItemExists(runID, sourcePath) {
		if archiveErr := archiveFailedInboxSource(v, archive, runID, result); archiveErr != nil {
			result.Reason = fmt.Sprintf("%s; archive failed: %s", result.Reason, archiveErr.Error())
		}
	}
	return result
}

func (r Runner) processInboxSource(ctx context.Context, v vault, archive archivepkg.Store, runID string, options Options, sourcePath string, shorthandPrompt string, progress ProgressFunc, locks *inboxRunLocks) (InboxSourceResult, error) {
	const totalStages = 6

	reportInboxStage(progress, sourcePath, "checking exact duplicates", 0, totalStages)
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
		locks.commit.Lock()
		defer locks.commit.Unlock()
		locks.archive.Lock()
		defer locks.archive.Unlock()
		return processExactDuplicateInboxSource(v, archive, runID, options, sourcePath, sourceContent, destinationPath)
	}

	reportInboxStage(progress, sourcePath, "embedding source and finding candidates", 1, totalStages)
	similar, err := indexer.New(v, options, r.embeddingProvider).Similar(ctx, sourcePath, sourceContent)
	if err != nil {
		return result, err
	}

	reportInboxStage(progress, sourcePath, "merging final destination notes", 2, totalStages)
	decision, err := r.buildInboxFinalNotes(ctx, archive, runID, sourcePath, sourceContent, similar, options, shorthandPrompt, &locks.archive)
	if err != nil {
		return result, err
	}
	locks.commit.Lock()
	defer locks.commit.Unlock()
	decision = constrainDecisionToCandidates(decision, similar)
	if err := ensureInboxDestinationsCurrent(v, options, decision, similar); err != nil {
		decision = pendingInboxDecision(sourcePath, err.Error())
	}
	claims := decision.Claims
	result.Claims = claims
	if len(claims) == 0 {
		result.Status = inboxStatusPending
		result.Reason = "no factual claims extracted"
		reportInboxStage(progress, sourcePath, "archiving pending result", 3, totalStages)
		locks.archive.Lock()
		defer locks.archive.Unlock()
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
	destinationWrites := applied.destinationWrites
	result.Ledger = ledger
	result.DestinationPaths = applied.destinationPaths
	result.Diffs = applied.destinationDiffs
	result.MergedCount, result.DedupedCount, result.PendingCount = countLedgerStatuses(ledger)
	if result.MergedCount+result.DedupedCount == 0 {
		result.Status = inboxStatusPending
		result.Reason = firstPendingReason(ledger, "one or more claims could not be safely merged or deduped")
		reportInboxStage(progress, sourcePath, "archiving pending result", 3, totalStages)
		locks.archive.Lock()
		defer locks.archive.Unlock()
		if _, err := archive.WriteInboxItem(runID, result, sourceContent, destinationBefore, destinationWrites); err != nil {
			return result, err
		}
		return result, nil
	}

	result.Status = inboxStatusProcessed
	if result.PendingCount > 0 {
		result.Status = inboxStatusPartial
		result.Reason = "partial merge applied; unresolved claims preserved in pending folder: " + firstPendingReason(ledger, "one or more claims could not be safely merged or deduped")
	}
	reportInboxStage(progress, sourcePath, "archiving merge result", 3, totalStages)
	locks.archive.Lock()
	if _, err := archive.WriteInboxItem(runID, result, sourceContent, destinationBefore, destinationWrites); err != nil {
		locks.archive.Unlock()
		return result, err
	}
	locks.archive.Unlock()
	reportInboxStage(progress, sourcePath, "writing destination notes", 4, totalStages)
	if err := writeDestinationNotes(v, options, destinationWrites); err != nil {
		return result, err
	}
	moveSource := moveInboxSourceToProcessed
	if result.Status == inboxStatusPartial {
		moveSource = moveInboxSourceToPending
	}
	reportInboxStage(progress, sourcePath, "moving source note", 5, totalStages)
	processedPath, err := moveSource(v, options, sourcePath)
	if err != nil {
		return result, err
	}
	result.ProcessedPath = processedPath
	locks.archive.Lock()
	defer locks.archive.Unlock()
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

func reportInboxNoteProgress(progress ProgressFunc, action string, completed int, total int, sourcePath string) {
	if progress == nil {
		return
	}
	progress(progressmodel.Units(
		fmt.Sprintf("%s inbox note %d/%d: %s", action, completed, total, filepath.Base(sourcePath)),
		completed,
		total,
		"note",
	))
}

func synchronizedInboxProgress(progress ProgressFunc, locks *inboxRunLocks) ProgressFunc {
	if progress == nil {
		return nil
	}
	return func(update progressmodel.Update) {
		locks.progress.Lock()
		defer locks.progress.Unlock()
		progress(update)
	}
}

func normalizedInboxWorkers(workers int, sourceCount int) int {
	if workers < 1 {
		workers = 1
	}
	if sourceCount > 0 && workers > sourceCount {
		return sourceCount
	}
	return workers
}

func shuffleSourceNotes(sourceNotes []string) {
	rng := rand.New(rand.NewSource(time.Now().UTC().UnixNano()))
	rng.Shuffle(len(sourceNotes), func(i int, j int) {
		sourceNotes[i], sourceNotes[j] = sourceNotes[j], sourceNotes[i]
	})
}

func appendUniquePath(paths []string, path string) []string {
	for _, existing := range paths {
		if existing == path {
			return paths
		}
	}
	return append(paths, path)
}
