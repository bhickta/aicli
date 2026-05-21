package zettel

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	progressmodel "github.com/bhickta/aicli/internal/progress"
	"github.com/bhickta/aicli/internal/provider"
	archivepkg "github.com/bhickta/aicli/internal/workflow/zettel/archive"
)

type Service struct {
	candidateProvider  provider.Provider
	mergeProvider      provider.Provider
	validationProvider provider.Provider
	embeddingProvider  provider.Provider
}

func New(p provider.Provider) *Service {
	return NewWithEmbedding(p, p)
}

func NewWithEmbedding(p provider.Provider, embeddingProvider provider.Provider) *Service {
	return NewWithProviders(p, p, p, embeddingProvider)
}

func NewWithProviders(
	candidateProvider provider.Provider,
	mergeProvider provider.Provider,
	validationProvider provider.Provider,
	embeddingProvider provider.Provider,
) *Service {
	return &Service{
		candidateProvider:  candidateProvider,
		mergeProvider:      mergeProvider,
		validationProvider: validationProvider,
		embeddingProvider:  embeddingProvider,
	}
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
		progress(progressmodel.Indeterminate("finding similar zettelkasten notes"))
	}
	similar, err := newEmbeddingIndex(v, options, s.embeddingProvider).Similar(ctx, activePath, activeContent)
	if err != nil {
		return SuggestResponse{}, err
	}
	if progress != nil {
		progress(progressmodel.Indeterminate("judging candidate line ranges"))
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
		progress(progressmodel.Indeterminate("extracting approved source ranges"))
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
	proposal.ActiveMarkdown = activeContent
	proposal.SourceExtractions = extractions
	proposal.Models = ProposalModels{
		Judge:           options.CandidateModel,
		CandidateJudge:  options.CandidateModel,
		Merge:           options.MergeModel,
		ValidationJudge: options.ValidationModel,
		Embedding:       options.EmbeddingModel,
	}
	proposal.Providers = ProposalProviders{
		CandidateJudge:  providerID(s.candidateProvider),
		Merge:           providerID(s.mergeProvider),
		ValidationJudge: providerID(s.validationProvider),
		Embedding:       providerID(s.embeddingProvider),
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
	activeAbs, err := v.NotePath(proposal.ActivePath, options)
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
		sourceAbs, err := v.NotePath(extraction.Path, options)
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
		progress(progressmodel.Units("archiving originals", 0, 2, "operation"))
	}
	archive := archivepkg.NewStore(v, options)
	archivePath, err := archive.WriteBeforeApply(proposal, activeContent, sourceContents)
	if err != nil {
		return ApplyResponse{}, err
	}
	if progress != nil {
		progress(progressmodel.Units("applying merged note and clipping source ranges", 1, 2, "operation"))
	}
	if err := os.WriteFile(activeAbs, []byte(ensureTrailingNewline(proposal.FinalMarkdown)), 0o600); err != nil {
		return ApplyResponse{}, fmt.Errorf("write active note: %w", err)
	}
	for _, extraction := range proposal.SourceExtractions {
		sourceAbs, err := v.NotePath(extraction.Path, options)
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
		progress(progressmodel.Indeterminate("restoring latest zettel merge archive"))
	}
	jobID, err := archivepkg.NewStore(v, options).Rollback(req.JobID)
	if err != nil {
		return RollbackResponse{}, err
	}
	return RollbackResponse{JobID: jobID}, nil
}

func providerID(p provider.Provider) string {
	if p == nil {
		return ""
	}
	return p.ID()
}
