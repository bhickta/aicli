package manual

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bhickta/aicli/internal/provider"
)

func judgeCandidatesPrompt(activePath string, activeContent string, candidates []scoredCandidate, options Options) []provider.Message {
	payload := make([]map[string]any, 0, len(candidates))
	for i, candidate := range candidates {
		excerpt, _ := numberedExcerpt(candidate.Path, candidate.Content, options.CandidateJudgeChars)
		payload = append(payload, map[string]any{
			"id":               i + 1,
			"path":             candidate.Path,
			"similarity":       candidate.Similarity,
			"numbered_excerpt": excerpt,
		})
	}
	user, _ := json.MarshalIndent(map[string]any{
		"active_note": compactNote(activePath, activeContent, options.CandidateJudgeChars),
		"candidates":  payload,
		"required_schema": map[string]any{
			"decisions": []map[string]any{{
				"path":         "candidate path",
				"action":       "merge or skip",
				"confidence":   "number 0..1",
				"relationship": "duplicate, same_concept_fragment, direct_subsection, definition_expansion, broad_context, taxonomy, example_only, separate_concept, or topic_mismatch",
				"risk":         "low, medium, or high",
				"reason":       "short reason",
				"source_line_ranges": []map[string]any{{
					"start_line": "1-based inclusive start line from the candidate numbered_excerpt",
					"end_line":   "1-based inclusive end line from the candidate numbered_excerpt",
					"reason":     "why these exact lines should be extracted",
				}},
			}},
		},
	}, "", "  ")
	return []provider.Message{
		{Role: "system", Content: candidateJudgeSystemPrompt()},
		{Role: "user", Content: string(user)},
	}
}

func candidateJudgeSystemPrompt() string {
	return strings.Join([]string{
		"You are a broad-topic Zettelkasten mergeability judge.",
		"Return JSON only.",
		"The active note is a seed for a larger UPSC master note or chapter-level note.",
		"Use action=merge only when exact candidate lines naturally fit inside the active note without turning it into a different topic.",
		"Use action=skip for unrelated concepts, accidental keyword overlap, different institutions, different policy areas, or topic mismatch.",
		"Do not suggest links. This workflow only merges or skips.",
		"Candidate notes are provided as numbered_excerpt blocks with 1-based line numbers.",
		"For every action=merge decision, include source_line_ranges. Each range must use exact inclusive line numbers from that candidate's numbered_excerpt.",
		"Extract only the lines that should actually be merged into the active note. Leave unrelated lines out.",
		"If no exact candidate lines should be extracted, return action=skip.",
	}, "\n")
}

func mergeMessages(activePath string, targetContent string, sourceExtractions []SourceExtraction, options Options, retryHint string) ([]provider.Message, error) {
	sections := []string{
		"ACTIVE NOTE PATH: " + activePath,
		"ACTIVE NOTE CONTENT WITH LINE NUMBERS:",
		numberedNote(activePath, targetContent),
		"",
		"SOURCE LINES TO MERGE INTO ACTIVE NOTE:",
	}
	for i, extraction := range sourceExtractions {
		sections = append(
			sections,
			"",
			fmt.Sprintf("--- SOURCE %d: %s ---", i+1, extraction.Path),
			"EXTRACTED RANGES: "+formatRanges(extraction.SourceLineRanges),
			"EXTRACTED MARKDOWN:",
			extraction.ExtractedMarkdown,
		)
	}
	sections = append(sections, "", "REQUIRED JSON SCHEMA:")
	schema, _ := json.MarshalIndent(MergePlan{
		Insertions: []Insertion{{
			AfterLine: 0,
			Markdown:  "markdown to insert; include only new material needed from extracted lines",
			Reason:    "short reason",
		}},
		Notes: "short explanation",
	}, "", "  ")
	sections = append(sections, string(schema))
	if retryHint != "" {
		sections = append(sections, "", "RETRY REQUIREMENTS:", retryHint)
	}
	user := strings.Join(sections, "\n")
	if len(user) > options.MaxMergeInputChars {
		return nil, fmt.Errorf("merge input is %d characters, above max_merge_input_chars=%d", len(user), options.MaxMergeInputChars)
	}
	return []provider.Message{
		{Role: "system", Content: mergeSystemPrompt()},
		{Role: "user", Content: user},
	}, nil
}

func mergeSystemPrompt() string {
	return strings.Join([]string{
		"You are a scoped Zettelkasten merge planner.",
		"Return JSON only.",
		"You receive the active note with 1-based line numbers and extracted source lines.",
		"Do not rewrite the active note.",
		"Return insertion operations only. Each insertion adds Markdown after an existing active-note line.",
		"Use after_line=0 only when the insertion belongs at the start of the active note.",
		"Do not edit, reorder, paraphrase, or remove existing active-note lines.",
		"Every source fact, date, number, name, qualifier, tag, wikilink, quote, and meaningful detail from the extracted lines must be present in the final note unless the active note already contains it.",
		"Do not use material from source lines that were not extracted.",
		"Do not add external knowledge.",
	}, "\n")
}

func validationMessages(sourceMaterial string, finalContent string) []provider.Message {
	payload, _ := json.Marshal(map[string]any{
		"source_notes":          sourceMaterial,
		"candidate_merged_note": finalContent,
		"required_schema": map[string]any{
			"verdict":               "pass or fail",
			"score":                 "number 0..1",
			"missing_facts":         []string{"facts missing from candidate"},
			"unsupported_additions": []string{"facts in candidate not supported by source"},
			"notes":                 "short explanation",
		},
	})
	return []provider.Message{
		{Role: "system", Content: strings.Join([]string{
			"You are a strict no-loss merge verifier.",
			"Compare source notes against the candidate merged note.",
			"Fail if any source fact, date, number, name, qualifier, tag, wikilink, or meaningful detail is missing.",
			"Fail if the candidate adds unsupported external knowledge.",
			"Return JSON only.",
		}, "\n")},
		{Role: "user", Content: string(payload)},
	}
}

func formatRanges(ranges []LineRange) string {
	parts := make([]string, 0, len(ranges))
	for _, r := range ranges {
		if r.StartLine == r.EndLine {
			parts = append(parts, fmt.Sprintf("%d", r.StartLine))
		} else {
			parts = append(parts, fmt.Sprintf("%d-%d", r.StartLine, r.EndLine))
		}
	}
	return strings.Join(parts, ", ")
}

func retryHint(coverage CoverageReport, judge MergeJudge) string {
	var parts []string
	if len(coverage.MissingLinks) > 0 {
		parts = append(parts, "Missing wikilinks: "+strings.Join(coverage.MissingLinks, ", "))
	}
	if len(coverage.MissingTags) > 0 {
		parts = append(parts, "Missing tags: "+strings.Join(coverage.MissingTags, ", "))
	}
	if len(coverage.MissingDates) > 0 {
		parts = append(parts, "Missing dates: "+strings.Join(coverage.MissingDates, ", "))
	}
	if len(coverage.MissingNumbers) > 0 {
		parts = append(parts, "Missing numbers: "+strings.Join(coverage.MissingNumbers, ", "))
	}
	if len(coverage.MissingHeadings) > 0 {
		parts = append(parts, "Missing headings: "+strings.Join(coverage.MissingHeadings, ", "))
	}
	if len(judge.MissingFacts) > 0 {
		parts = append(parts, "Judge missing facts: "+strings.Join(judge.MissingFacts, " | "))
	}
	return strings.Join(parts, "\n")
}
