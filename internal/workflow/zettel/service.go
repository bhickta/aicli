package zettel

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/bhickta/aicli/internal/provider"
)

type Service struct {
	provider          provider.Provider
	embeddingProvider provider.Provider
}

func New(p provider.Provider) *Service {
	return NewWithEmbedding(p, p)
}

func NewWithEmbedding(p provider.Provider, embeddingProvider provider.Provider) *Service {
	return &Service{provider: p, embeddingProvider: embeddingProvider}
}

func (s *Service) Index(ctx context.Context, req IndexRequest, progress ProgressFunc) (IndexResponse, error) {
	options := normalizeOptions(req.Options)
	v, err := newVault(options.VaultPath)
	if err != nil {
		return IndexResponse{}, err
	}
	return newEmbeddingIndex(v, options, s.embeddingProvider).Build(ctx, progress)
}

func (s *Service) Suggest(ctx context.Context, req SuggestRequest, progress ProgressFunc) (SuggestResponse, error) {
	options := normalizeOptions(req.Options)
	v, activePath, activeContent, err := readActive(options, req.ActivePath)
	if err != nil {
		return SuggestResponse{}, err
	}
	if progress != nil {
		progress("finding similar zettelkasten notes", 2, 5)
	}
	similar, err := newEmbeddingIndex(v, options, s.embeddingProvider).Similar(ctx, activePath, activeContent)
	if err != nil {
		return SuggestResponse{}, err
	}
	if progress != nil {
		progress("judging candidate line ranges", 3, 5)
	}
	candidates, err := s.judgeCandidates(ctx, activePath, activeContent, similar, options)
	if err != nil {
		return SuggestResponse{}, err
	}
	return SuggestResponse{
		ActivePath: activePath,
		ActiveHash: hashText(activeContent),
		Candidates: candidates,
	}, nil
}

func (s *Service) Propose(ctx context.Context, req ProposeRequest, progress ProgressFunc) (ProposeResponse, error) {
	options := normalizeOptions(req.Options)
	v, activePath, activeContent, err := readActive(options, req.ActivePath)
	if err != nil {
		return ProposeResponse{}, err
	}
	if len(req.Selections) == 0 {
		return ProposeResponse{}, errors.New("at least one selected candidate is required")
	}
	if progress != nil {
		progress("extracting approved source ranges", 2, 6)
	}
	extractions, sourceMaterial, err := buildExtractions(v, options, req.Selections)
	if err != nil {
		return ProposeResponse{}, err
	}
	sourceMaterial = activeContent + "\n\n--- SOURCE BREAK ---\n\n" + sourceMaterial
	proposal, err := s.runMergeAttempts(ctx, activePath, activeContent, extractions, sourceMaterial, options, progress)
	if err != nil {
		return ProposeResponse{}, err
	}
	proposal.ID = fmt.Sprintf("zettel-%d", time.Now().UTC().UnixNano())
	proposal.CreatedAt = time.Now().UTC()
	proposal.VaultPath = options.VaultPath
	proposal.RootFolder = options.RootFolder
	proposal.DataFolder = options.DataFolder
	proposal.ActivePath = activePath
	proposal.ActiveHash = hashText(activeContent)
	proposal.SourceExtractions = extractions
	proposal.Models = ProposalModels{
		Judge:     options.JudgeModel,
		Merge:     options.MergeModel,
		Embedding: options.EmbeddingModel,
	}
	return ProposeResponse{Proposal: proposal}, nil
}

func (s *Service) Apply(_ context.Context, req ApplyRequest, progress ProgressFunc) (ApplyResponse, error) {
	options := normalizeOptions(req.Options)
	proposal := req.Proposal
	if proposal.ID == "" {
		return ApplyResponse{}, errors.New("proposal id is required")
	}
	v, err := newVault(options.VaultPath)
	if err != nil {
		return ApplyResponse{}, err
	}
	activeAbs, err := v.notePath(proposal.ActivePath, options)
	if err != nil {
		return ApplyResponse{}, err
	}
	activeContentBytes, err := os.ReadFile(activeAbs)
	if err != nil {
		return ApplyResponse{}, fmt.Errorf("read active note: %w", err)
	}
	activeContent := string(activeContentBytes)
	if hashText(activeContent) != proposal.ActiveHash {
		return ApplyResponse{}, fmt.Errorf("active note changed before apply: %s", proposal.ActivePath)
	}
	sourceContents := make(map[string]string, len(proposal.SourceExtractions))
	sourcePaths := make([]string, 0, len(proposal.SourceExtractions))
	for _, extraction := range proposal.SourceExtractions {
		sourceAbs, err := v.notePath(extraction.Path, options)
		if err != nil {
			return ApplyResponse{}, err
		}
		content, err := os.ReadFile(sourceAbs)
		if err != nil {
			return ApplyResponse{}, fmt.Errorf("read source note %s: %w", extraction.Path, err)
		}
		if hashText(string(content)) != extraction.OriginalHash {
			return ApplyResponse{}, fmt.Errorf("source note changed before apply: %s", extraction.Path)
		}
		sourceContents[extraction.Path] = string(content)
		sourcePaths = append(sourcePaths, extraction.Path)
	}
	if progress != nil {
		progress("archiving originals", 2, 4)
	}
	archive := newArchiveStore(v, options)
	archivePath, err := archive.WriteBeforeApply(proposal, activeContent, sourceContents)
	if err != nil {
		return ApplyResponse{}, err
	}
	if progress != nil {
		progress("applying merged note and clipping source ranges", 3, 4)
	}
	if err := os.WriteFile(activeAbs, []byte(ensureTrailingNewline(proposal.FinalMarkdown)), 0o600); err != nil {
		return ApplyResponse{}, fmt.Errorf("write active note: %w", err)
	}
	for _, extraction := range proposal.SourceExtractions {
		sourceAbs, err := v.notePath(extraction.Path, options)
		if err != nil {
			return ApplyResponse{}, err
		}
		remaining := removeLineRanges(sourceContents[extraction.Path], extraction.SourceLineRanges)
		if err := os.WriteFile(sourceAbs, []byte(ensureTrailingNewline(remaining)), 0o600); err != nil {
			return ApplyResponse{}, fmt.Errorf("write source note %s: %w", extraction.Path, err)
		}
	}
	if err := archive.MarkApplied(proposal.ID); err != nil {
		return ApplyResponse{}, err
	}
	return ApplyResponse{JobID: proposal.ID, ActivePath: proposal.ActivePath, SourcePaths: sourcePaths, ArchivePath: archivePath}, nil
}

func (s *Service) Rollback(_ context.Context, req RollbackRequest, progress ProgressFunc) (RollbackResponse, error) {
	options := normalizeOptions(req.Options)
	v, err := newVault(options.VaultPath)
	if err != nil {
		return RollbackResponse{}, err
	}
	if progress != nil {
		progress("restoring latest zettel merge archive", 2, 4)
	}
	jobID, err := newArchiveStore(v, options).Rollback(req.JobID)
	if err != nil {
		return RollbackResponse{}, err
	}
	return RollbackResponse{JobID: jobID}, nil
}
