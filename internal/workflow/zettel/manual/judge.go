package manual

import (
	"context"
	"fmt"
	"strings"

	progressmodel "github.com/bhickta/aicli/internal/progress"
	"github.com/bhickta/aicli/internal/workflow/zettel/audit"
	"github.com/bhickta/aicli/internal/workflow/zettel/llmjson"
)

func (r Runner) judgeCandidates(ctx context.Context, activePath string, activeContent string, similar []scoredCandidate, options Options) ([]Candidate, error) {
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
	response, err := llmjson.Chat[struct {
		Decisions []decision `json:"decisions"`
	}](ctx, r.candidateProvider, options.CandidateModel, judgeCandidatesPrompt(activePath, activeContent, similar, options))
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

func (r Runner) runMergeAttempts(ctx context.Context, activePath string, activeContent string, extractions []SourceExtraction, sourceMaterial string, options Options, progress ProgressFunc) (Proposal, error) {
	var proposal Proposal
	var hint string
	for attempt := 1; attempt <= options.MaxMergeRetries; attempt++ {
		if progress != nil {
			progress(progressmodel.Indeterminate(fmt.Sprintf("building merge proposal %d/%d", attempt, options.MaxMergeRetries)))
		}
		messages, err := mergeMessages(activePath, activeContent, extractions, options, hint)
		if err != nil {
			return proposal, err
		}
		plan, err := llmjson.Chat[MergePlan](ctx, r.mergeProvider, options.MergeModel, messages)
		if err != nil {
			return proposal, err
		}
		plan = normalizeMergePlan(plan, len(splitLines(activeContent)))
		finalContent := applyMergePlan(activeContent, plan)
		if progress != nil {
			progress(progressmodel.Indeterminate(fmt.Sprintf("validating merge proposal %d/%d", attempt, options.MaxMergeRetries)))
		}
		coverage := audit.BuildCoverageReport(sourceMaterial, finalContent)
		judge, err := llmjson.Chat[MergeJudge](ctx, r.validationProvider, options.ValidationModel, validationMessages(sourceMaterial, finalContent))
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
