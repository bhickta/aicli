package zettel

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
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

func readActive(options Options, path string) (vault, string, string, error) {
	v, err := newVault(options.VaultPath)
	if err != nil {
		return vault{}, "", "", err
	}
	abs, err := v.notePath(path, options)
	if err != nil {
		return vault{}, "", "", err
	}
	rel, err := v.rel(abs)
	if err != nil {
		return vault{}, "", "", err
	}
	content, err := os.ReadFile(abs)
	if err != nil {
		return vault{}, "", "", fmt.Errorf("read active note: %w", err)
	}
	return v, rel, string(content), nil
}

func buildExtractions(v vault, options Options, selections []Selection) ([]SourceExtraction, string, error) {
	extractions := make([]SourceExtraction, 0, len(selections))
	sourceBlocks := make([]string, 0, len(selections))
	for _, selection := range selections {
		abs, err := v.notePath(selection.Path, options)
		if err != nil {
			return nil, "", err
		}
		rel, err := v.rel(abs)
		if err != nil {
			return nil, "", err
		}
		contentBytes, err := os.ReadFile(abs)
		if err != nil {
			return nil, "", fmt.Errorf("read source note %s: %w", rel, err)
		}
		content := string(contentBytes)
		ranges, err := normalizeRanges(selection.SourceLineRanges, content, len(splitLines(content)))
		if err != nil {
			return nil, "", fmt.Errorf("source note %s: %w", rel, err)
		}
		extracted := extractLineRanges(content, ranges)
		if strings.TrimSpace(extracted) == "" {
			return nil, "", fmt.Errorf("source note %s selected ranges are empty", rel)
		}
		extractions = append(extractions, SourceExtraction{
			Path:              rel,
			OriginalHash:      hashText(content),
			SourceLineRanges:  ranges,
			ExtractedMarkdown: extracted,
		})
		sourceBlocks = append(sourceBlocks, extracted)
	}
	return extractions, strings.Join(sourceBlocks, "\n\n--- SOURCE BREAK ---\n\n"), nil
}

func normalizeRanges(ranges []LineRange, content string, upperLine int) ([]LineRange, error) {
	if len(ranges) == 0 {
		return nil, errors.New("source line ranges are required")
	}
	totalLines := len(splitLines(content))
	if upperLine <= 0 || upperLine > totalLines {
		upperLine = totalLines
	}
	normalized := mergeLineRanges(ranges)
	for _, r := range normalized {
		if r.StartLine < 1 || r.EndLine < r.StartLine || r.EndLine > upperLine {
			return nil, fmt.Errorf("invalid line range %d-%d", r.StartLine, r.EndLine)
		}
	}
	return normalized, nil
}

func (s *Service) judgeCandidates(ctx context.Context, activePath string, activeContent string, similar []scoredCandidate, options Options) ([]Candidate, error) {
	if len(similar) == 0 {
		return []Candidate{}, nil
	}
	type decision struct {
		Path             string      `json:"path"`
		Action           string      `json:"action"`
		Confidence       float64     `json:"confidence"`
		Relationship     string      `json:"relationship"`
		Risk             string      `json:"risk"`
		Reason           string      `json:"reason"`
		SourceLineRanges []LineRange `json:"source_line_ranges"`
	}
	var response struct {
		Decisions []decision `json:"decisions"`
	}
	response, err := chatJSON[struct {
		Decisions []decision `json:"decisions"`
	}](ctx, s.provider, options.JudgeModel, judgeCandidatesPrompt(activePath, activeContent, similar, options))
	if err != nil {
		return nil, err
	}
	byPath := map[string]scoredCandidate{}
	for _, candidate := range similar {
		byPath[candidate.Path] = candidate
	}
	out := make([]Candidate, 0, len(response.Decisions))
	for _, raw := range response.Decisions {
		if raw.Action != "merge" || raw.Confidence < options.ReviewThreshold {
			continue
		}
		source, ok := byPath[raw.Path]
		if !ok {
			continue
		}
		_, visibleLineLimit := numberedExcerpt(source.Path, source.Content, options.CandidateJudgeChars)
		ranges, err := normalizeRanges(raw.SourceLineRanges, source.Content, visibleLineLimit)
		if err != nil {
			continue
		}
		out = append(out, Candidate{
			Path:              source.Path,
			Similarity:        source.Similarity,
			Confidence:        raw.Confidence,
			Relationship:      normalizeRelationship(raw.Relationship),
			Risk:              normalizeRisk(raw.Risk),
			Reason:            raw.Reason,
			SourceLineRanges:  ranges,
			ExtractedMarkdown: extractLineRanges(source.Content, ranges),
		})
	}
	return out, nil
}

func (s *Service) runMergeAttempts(ctx context.Context, activePath string, activeContent string, extractions []SourceExtraction, sourceMaterial string, options Options, progress ProgressFunc) (Proposal, error) {
	var proposal Proposal
	var hint string
	for attempt := 1; attempt <= options.MaxMergeRetries; attempt++ {
		if progress != nil {
			progress(fmt.Sprintf("building merge proposal %d/%d", attempt, options.MaxMergeRetries), 3, 6)
		}
		messages, err := mergeMessages(activePath, activeContent, extractions, options, hint)
		if err != nil {
			return proposal, err
		}
		plan, err := chatJSON[MergePlan](ctx, s.provider, options.MergeModel, messages)
		if err != nil {
			return proposal, err
		}
		plan = normalizeMergePlan(plan, len(splitLines(activeContent)))
		finalContent := applyMergePlan(activeContent, plan)
		if progress != nil {
			progress(fmt.Sprintf("validating merge proposal %d/%d", attempt, options.MaxMergeRetries), 4, 6)
		}
		coverage := buildCoverageReport(sourceMaterial, finalContent)
		judge, err := chatJSON[MergeJudge](ctx, s.provider, options.JudgeModel, validationMessages(sourceMaterial, finalContent))
		if err != nil {
			return proposal, err
		}
		judge = normalizeJudge(judge)
		proposal.MergePlan = plan
		proposal.FinalMarkdown = finalContent
		proposal.Coverage = coverage
		proposal.Judge = judge
		if isValidated(coverage, judge, options.ValidationThreshold) {
			return proposal, nil
		}
		hint = retryHint(coverage, judge)
		if hint == "" {
			hint = "The validator failed. Re-check all source facts and preserve every missing or transformed detail."
		}
	}
	return proposal, fmt.Errorf("merge proposal failed validation: %s", strings.TrimSpace(proposal.Judge.Notes))
}

func normalizeMergePlan(plan MergePlan, lineCount int) MergePlan {
	out := MergePlan{Notes: strings.TrimSpace(plan.Notes)}
	for _, insertion := range plan.Insertions {
		markdown := strings.Trim(insertion.Markdown, "\n")
		if strings.TrimSpace(markdown) == "" {
			continue
		}
		afterLine := insertion.AfterLine
		if afterLine < 0 {
			afterLine = 0
		}
		if afterLine > lineCount {
			afterLine = lineCount
		}
		out.Insertions = append(out.Insertions, Insertion{
			AfterLine: afterLine,
			Markdown:  markdown,
			Reason:    strings.TrimSpace(insertion.Reason),
		})
	}
	return out
}

func normalizeJudge(judge MergeJudge) MergeJudge {
	if judge.Verdict != "pass" {
		judge.Verdict = "fail"
	}
	return judge
}

func isValidated(coverage CoverageReport, judge MergeJudge, threshold float64) bool {
	return coverage.RequiredMissingCount == 0 &&
		coverage.Score >= threshold &&
		judge.Verdict == "pass" &&
		judge.Score >= threshold &&
		len(judge.UnsupportedAdditions) == 0
}

func normalizeRelationship(value string) string {
	switch value {
	case "duplicate", "same_concept_fragment", "direct_subsection", "definition_expansion", "broad_context", "taxonomy", "example_only", "separate_concept", "topic_mismatch":
		return value
	default:
		return "separate_concept"
	}
}

func normalizeRisk(value string) string {
	switch value {
	case "medium", "high":
		return value
	default:
		return "low"
	}
}
