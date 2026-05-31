package metadata

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	progressmodel "github.com/bhickta/aicli/internal/progress"
	"github.com/bhickta/aicli/internal/provider"
	archivepkg "github.com/bhickta/aicli/internal/workflow/zettel/archive"
	"github.com/bhickta/aicli/internal/workflow/zettel/model"
	"github.com/bhickta/aicli/internal/workflow/zettel/notetext"
	"github.com/bhickta/aicli/internal/workflow/zettel/vaultfs"
)

func (r Runner) Generate(ctx context.Context, req MetadataRequest, progress ProgressFunc) (MetadataResponse, error) {
	options := model.NormalizeOptions(req.Options)
	v, err := vaultfs.New(options.VaultPath)
	if err != nil {
		return MetadataResponse{}, err
	}
	folder, err := normalizeMetadataFolder(v, req.MetadataFolder, options.RootFolder)
	if err != nil {
		return MetadataResponse{}, err
	}
	scanOptions := options
	scanOptions.RootFolder = folder
	notes, err := v.ScanNotes(scanOptions)
	if err != nil {
		return MetadataResponse{}, err
	}
	sort.Strings(notes)
	sourceCount := len(notes)
	limit := normalizeMetadataLimit(req.MetadataLimit)
	if limit > 0 && limit < len(notes) {
		notes = notes[:limit]
	}

	runID := fmt.Sprintf("zettel-metadata-%d", time.Now().UTC().UnixNano())
	archive := archivepkg.NewStore(v, options)
	archivePath, err := archive.MetadataRunPath(runID)
	if err != nil {
		return MetadataResponse{}, err
	}
	response := MetadataResponse{
		RunID:         runID,
		ArchivePath:   archivePath,
		SourceCount:   sourceCount,
		SelectedCount: len(notes),
		SkippedCount:  sourceCount - len(notes),
		Limit:         limit,
	}
	if len(notes) == 0 {
		return response, nil
	}

	results := r.processMetadataNotes(ctx, v, archive, runID, options, notes, req.MetadataOverwrite, req.MetadataWorkers, progress)
	for _, result := range results {
		switch result.Status {
		case StatusProcessed:
			response.Processed = append(response.Processed, result)
		case StatusFailed:
			response.Failed = append(response.Failed, result)
		default:
			if result.Status == "" {
				result.Status = StatusSkipped
			}
			response.Skipped = append(response.Skipped, result)
		}
	}
	if progress != nil {
		progress(progressmodel.Units("completed metadata run", len(notes), len(notes), "note"))
	}
	response.ProcessedCount = len(response.Processed)
	response.FailedCount = len(response.Failed)
	if err := archive.FinalizeMetadataRun(runID, response); err != nil {
		return response, err
	}
	return response, nil
}

type metadataRunLocks struct {
	archive  sync.Mutex
	progress sync.Mutex
}

type metadataJob struct {
	index int
	path  string
}

type metadataOutcome struct {
	index  int
	result MetadataNoteResult
}

func (r Runner) processMetadataNotes(
	ctx context.Context,
	v vault,
	archive archivepkg.Store,
	runID string,
	options Options,
	notes []string,
	overwrite bool,
	workerCount int,
	progress ProgressFunc,
) []MetadataNoteResult {
	workers := normalizedMetadataWorkers(workerCount, len(notes))
	locks := &metadataRunLocks{}
	progress = synchronizedMetadataProgress(progress, locks)
	if workers <= 1 {
		results := make([]MetadataNoteResult, 0, len(notes))
		for i, path := range notes {
			reportMetadataProgress(progress, "generating metadata", i, len(notes), path)
			result := r.processMetadataJob(ctx, v, archive, runID, options, path, overwrite, &locks.archive)
			results = append(results, result)
			reportMetadataProgress(progress, "finished metadata", i+1, len(notes), path)
		}
		return results
	}

	jobs := make(chan metadataJob)
	outcomes := make(chan metadataOutcome, len(notes))
	var completed atomic.Int64
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				reportMetadataProgress(progress, "generating metadata", int(completed.Load()), len(notes), job.path)
				result := r.processMetadataJob(ctx, v, archive, runID, options, job.path, overwrite, &locks.archive)
				done := int(completed.Add(1))
				reportMetadataProgress(progress, "finished metadata", done, len(notes), job.path)
				outcomes <- metadataOutcome{index: job.index, result: result}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for i, path := range notes {
			select {
			case <-ctx.Done():
				return
			case jobs <- metadataJob{index: i, path: path}:
			}
		}
	}()
	go func() {
		wg.Wait()
		close(outcomes)
	}()

	results := make([]MetadataNoteResult, len(notes))
	for outcome := range outcomes {
		results[outcome.index] = outcome.result
	}
	for i, result := range results {
		if result.Path == "" {
			reason := "metadata run stopped before this note completed"
			if err := ctx.Err(); err != nil {
				reason = err.Error()
			}
			results[i] = MetadataNoteResult{Path: notes[i], Status: StatusFailed, Reason: reason}
		}
	}
	return results
}

func (r Runner) processMetadataJob(
	ctx context.Context,
	v vault,
	archive archivepkg.Store,
	runID string,
	options Options,
	path string,
	overwrite bool,
	archiveMu *sync.Mutex,
) MetadataNoteResult {
	result, before, after, err := r.processMetadataNote(ctx, v, archive, runID, options, path, overwrite, archiveMu)
	if err != nil {
		result = MetadataNoteResult{Path: path, Status: StatusFailed, Reason: err.Error()}
	}
	if archiveMu != nil {
		archiveMu.Lock()
		defer archiveMu.Unlock()
	}
	if _, archiveErr := archive.WriteMetadataItem(runID, result, before, after); archiveErr != nil {
		if result.Reason == "" {
			result.Reason = archiveErr.Error()
		} else {
			result.Reason = result.Reason + "; archive failed: " + archiveErr.Error()
		}
		result.Status = StatusFailed
	}
	return result
}

func (r Runner) processMetadataNote(
	ctx context.Context,
	v vault,
	archive archivepkg.Store,
	runID string,
	options Options,
	path string,
	overwrite bool,
	archiveMu *sync.Mutex,
) (MetadataNoteResult, string, string, error) {
	abs, err := v.Abs(path)
	if err != nil {
		return MetadataNoteResult{}, "", "", err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return MetadataNoteResult{}, "", "", fmt.Errorf("read metadata note: %w", err)
	}
	before := string(data)
	frontmatter, _, _ := splitFrontmatter(before)
	if hasCompleteMetadata(frontmatter) && !overwrite {
		return MetadataNoteResult{
			Path:   path,
			Status: StatusSkipped,
			Reason: "metadata already exists",
		}, before, before, nil
	}
	if !hasSubstantiveBody(before) {
		return MetadataNoteResult{
			Path:   path,
			Status: StatusSkipped,
			Reason: "no substantive body content",
		}, before, before, nil
	}

	body := markdownBody(before)
	item, err := r.generateMetadata(ctx, archive, runID, options, path, body, archiveMu)
	if err != nil {
		return MetadataNoteResult{}, before, before, err
	}
	after, changed, skipped := applyMetadata(before, item, overwrite)
	result := MetadataNoteResult{
		Path:            path,
		Status:          StatusProcessed,
		Title:           item.Title,
		SummaryKeywords: item.SummaryKeywords,
		RecallQuestions: item.RecallQuestions,
	}
	if skipped || !changed {
		result.Status = StatusSkipped
		result.Reason = "metadata already exists"
		return result, before, before, nil
	}
	result.Diff = &model.InboxDestinationDiff{
		Path:   path,
		Before: before,
		After:  after,
		Diff:   notetext.SimpleMarkdownDiff(before, after),
	}
	if err := os.WriteFile(abs, []byte(after), 0o600); err != nil {
		return result, before, after, fmt.Errorf("write metadata note: %w", err)
	}
	return result, before, after, nil
}

func (r Runner) generateMetadata(
	ctx context.Context,
	archive archivepkg.Store,
	runID string,
	options Options,
	path string,
	content string,
	archiveMu *sync.Mutex,
) (generatedMetadata, error) {
	if r.provider == nil {
		return generatedMetadata{}, errors.New("provider is required")
	}
	modelName := strings.TrimSpace(options.MergeModel)
	req := provider.ChatRequest{
		Model:       modelName,
		Messages:    metadataMessages(path, content),
		Temperature: 0,
	}
	res, err := r.provider.Chat(ctx, req)
	parsedFormat := "unparsed"
	var item generatedMetadata
	if err == nil {
		item, err = parseGeneratedMetadata(res.Content)
		if err == nil {
			parsedFormat = "metadata-json"
		} else {
			parsedFormat = "invalid-metadata-json"
		}
	}
	if archiveMu != nil {
		archiveMu.Lock()
		defer archiveMu.Unlock()
	}
	if _, traceErr := archive.WriteMetadataLLMExchange(runID, archivepkg.LLMExchange{
		Step:         "generate-metadata",
		SourcePath:   path,
		ProviderID:   providerID(r.provider),
		Model:        modelName,
		Request:      req,
		Response:     res,
		Error:        errorString(err),
		ParsedFormat: parsedFormat,
	}); traceErr != nil {
		if err != nil {
			return generatedMetadata{}, errors.Join(err, traceErr)
		}
		return generatedMetadata{}, traceErr
	}
	if err != nil {
		return generatedMetadata{}, err
	}
	return item, nil
}

func normalizeMetadataFolder(v vault, folder string, rootFolder string) (string, error) {
	folder = strings.TrimSpace(folder)
	if folder == "" {
		folder = rootFolder
	}
	if filepath.IsAbs(folder) {
		abs, err := v.Abs(folder)
		if err != nil {
			return "", err
		}
		rel, err := v.Rel(abs)
		if err != nil {
			return "", err
		}
		folder = rel
	}
	folder = strings.Trim(filepath.ToSlash(filepath.Clean(folder)), "/")
	if folder == "." || folder == "" {
		folder = rootFolder
	}
	return folder, nil
}

func normalizeMetadataLimit(limit int) int {
	if limit < 0 {
		return 0
	}
	return limit
}

func normalizedMetadataWorkers(workers int, noteCount int) int {
	if workers < 1 {
		workers = 1
	}
	if noteCount > 0 && workers > noteCount {
		return noteCount
	}
	return workers
}

func reportMetadataProgress(progress ProgressFunc, action string, completed int, total int, path string) {
	if progress == nil {
		return
	}
	progress(progressmodel.Units(
		fmt.Sprintf("%s %d/%d: %s", action, completed, total, filepath.Base(path)),
		completed,
		total,
		"note",
	))
}

func synchronizedMetadataProgress(progress ProgressFunc, locks *metadataRunLocks) ProgressFunc {
	if progress == nil {
		return nil
	}
	return func(update progressmodel.Update) {
		locks.progress.Lock()
		defer locks.progress.Unlock()
		progress(update)
	}
}

func providerID(p provider.Provider) string {
	if p == nil {
		return ""
	}
	return p.ID()
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
