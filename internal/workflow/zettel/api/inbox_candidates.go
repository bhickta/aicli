package zettel

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"time"

	progressmodel "github.com/bhickta/aicli/internal/progress"
	"github.com/bhickta/aicli/internal/workflow/zettel/indexer"
	"github.com/bhickta/aicli/internal/workflow/zettel/notetext"
	"github.com/bhickta/aicli/internal/workflow/zettel/vaultfs"
)

const (
	sourcePreviewExcerptChars    = 1600
	candidatePreviewExcerptChars = 1200
)

func (s *Service) InboxCandidatePreview(
	ctx context.Context,
	req InboxCandidatePreviewRequest,
	progress ProgressFunc,
) (InboxCandidatePreviewResponse, error) {
	options := s.workflowOptions(req.Options)
	v, err := vaultfs.New(options.VaultPath)
	if err != nil {
		return InboxCandidatePreviewResponse{}, err
	}
	sourceNotes, err := v.ScanInboxNotes(options)
	if err != nil {
		return InboxCandidatePreviewResponse{}, err
	}
	sourceNotes, sourceCount, skippedCount := selectInboxPreviewSources(sourceNotes, options)
	response := InboxCandidatePreviewResponse{
		SourceCount:   sourceCount,
		SelectedCount: len(sourceNotes),
		SkippedCount:  skippedCount,
		Limit:         options.InboxLimit,
		Sources:       []InboxCandidateSource{},
	}
	if len(sourceNotes) == 0 {
		return response, nil
	}

	tracker, _, embeddingProvider := s.trackedProviders()
	idx := indexer.New(v, options, embeddingProvider)
	for i, sourcePath := range sourceNotes {
		if err := ctx.Err(); err != nil {
			response.APICalls = tracker.Snapshot()
			return response, err
		}
		reportInboxCandidatePreviewProgress(progress, sourcePath, i, len(sourceNotes))
		source := InboxCandidateSource{
			SourcePath: sourcePath,
			Candidates: []InboxCandidate{},
		}
		sourceContent, err := readInboxPreviewSource(v, sourcePath)
		if err != nil {
			source.Error = err.Error()
			response.Sources = append(response.Sources, source)
			continue
		}
		source.SourceExcerpt = previewExcerpt(sourcePath, sourceContent, sourcePreviewExcerptChars)
		similar, err := idx.Similar(ctx, sourcePath, sourceContent)
		if err != nil {
			source.Error = err.Error()
			response.Sources = append(response.Sources, source)
			continue
		}
		source.Candidates = inboxPreviewCandidates(similar)
		response.Sources = append(response.Sources, source)
	}
	if progress != nil {
		progress(progressmodel.Units("completed embedding candidate preview", len(sourceNotes), len(sourceNotes), "note"))
	}
	response.APICalls = tracker.Snapshot()
	return response, nil
}

func selectInboxPreviewSources(sourceNotes []string, options Options) ([]string, int, int) {
	sort.Strings(sourceNotes)
	if options.InboxRandom {
		rng := rand.New(rand.NewSource(time.Now().UTC().UnixNano()))
		rng.Shuffle(len(sourceNotes), func(i int, j int) {
			sourceNotes[i], sourceNotes[j] = sourceNotes[j], sourceNotes[i]
		})
	}
	sourceCount := len(sourceNotes)
	if options.InboxLimit > 0 && options.InboxLimit < len(sourceNotes) {
		sourceNotes = sourceNotes[:options.InboxLimit]
	}
	return sourceNotes, sourceCount, sourceCount - len(sourceNotes)
}

func readInboxPreviewSource(v vaultfs.Vault, sourcePath string) (string, error) {
	sourceAbs, err := v.Abs(sourcePath)
	if err != nil {
		return "", err
	}
	sourceBytes, err := os.ReadFile(sourceAbs)
	if err != nil {
		return "", fmt.Errorf("read inbox source: %w", err)
	}
	return string(sourceBytes), nil
}

func inboxPreviewCandidates(candidates []indexer.ScoredCandidate) []InboxCandidate {
	out := make([]InboxCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		out = append(out, InboxCandidate{
			Path:       candidate.Path,
			Similarity: candidate.Similarity,
			Excerpt:    previewExcerpt(candidate.Path, candidate.Content, candidatePreviewExcerptChars),
		})
	}
	return out
}

func previewExcerpt(path string, content string, maxChars int) string {
	excerpt, _ := notetext.NumberedExcerpt(path, content, maxChars)
	return excerpt
}

func reportInboxCandidatePreviewProgress(progress ProgressFunc, sourcePath string, completed int, total int) {
	if progress == nil {
		return
	}
	progress(progressmodel.Units(
		fmt.Sprintf("previewing embedding candidates: %s", sourcePath),
		completed,
		total,
		"note",
	))
}
