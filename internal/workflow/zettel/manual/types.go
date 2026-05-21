package manual

import (
	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/workflow/zettel/indexer"
	"github.com/bhickta/aicli/internal/workflow/zettel/model"
	"github.com/bhickta/aicli/internal/workflow/zettel/notetext"
	"github.com/bhickta/aicli/internal/workflow/zettel/vaultfs"
)

type Options = model.Options
type SuggestRequest = model.SuggestRequest
type SuggestResponse = model.SuggestResponse
type Candidate = model.Candidate
type Selection = model.Selection
type ProposeRequest = model.ProposeRequest
type ProposeResponse = model.ProposeResponse
type ApplyRequest = model.ApplyRequest
type ApplyResponse = model.ApplyResponse
type RollbackRequest = model.RollbackRequest
type RollbackResponse = model.RollbackResponse
type Proposal = model.Proposal
type ProposalModels = model.ProposalModels
type ProposalProviders = model.ProposalProviders
type SourceExtraction = model.SourceExtraction
type LineRange = model.LineRange
type MergePlan = model.MergePlan
type Insertion = model.Insertion
type CoverageReport = model.CoverageReport
type MergeJudge = model.MergeJudge
type ProgressFunc = model.ProgressFunc

type vault = vaultfs.Vault
type scoredCandidate = indexer.ScoredCandidate

type Runner struct {
	candidateProvider  provider.Provider
	mergeProvider      provider.Provider
	validationProvider provider.Provider
	embeddingProvider  provider.Provider
}

func New(
	candidateProvider provider.Provider,
	mergeProvider provider.Provider,
	validationProvider provider.Provider,
	embeddingProvider provider.Provider,
) Runner {
	return Runner{
		candidateProvider:  candidateProvider,
		mergeProvider:      mergeProvider,
		validationProvider: validationProvider,
		embeddingProvider:  embeddingProvider,
	}
}

func normalizeOptions(options Options) Options {
	return model.NormalizeOptions(options)
}

func newVault(path string) (vault, error) {
	return vaultfs.New(path)
}

func splitLines(content string) []string {
	return notetext.SplitLines(content)
}

func hashText(content string) string {
	return notetext.HashText(content)
}

func numberedNote(path string, content string) string {
	return notetext.NumberedNote(path, content)
}

func numberedExcerpt(path string, content string, maxChars int) (string, int) {
	return notetext.NumberedExcerpt(path, content, maxChars)
}

func extractLineRanges(content string, ranges []LineRange) string {
	return notetext.ExtractLineRanges(content, ranges)
}

func removeLineRanges(content string, ranges []LineRange) string {
	return notetext.RemoveLineRanges(content, ranges)
}

func mergeLineRanges(ranges []LineRange) []LineRange {
	return notetext.MergeLineRanges(ranges)
}

func applyMergePlan(target string, plan MergePlan) string {
	return notetext.ApplyMergePlan(target, plan)
}

func compactNote(path string, content string, maxChars int) string {
	return notetext.CompactNote(path, content, maxChars)
}
