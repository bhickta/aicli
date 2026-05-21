package inbox

import (
	"encoding/json"
	"strings"

	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/workflow/zettel/notetext"
)

type inboxDestinationDecision struct {
	Claims       []InboxClaim                 `json:"claims,omitempty"`
	Destinations []inboxDestinationAssignment `json:"destinations"`
	Pending      []InboxClaimLedger           `json:"pending"`
	Validation   MergeJudge                   `json:"validation,omitempty"`
	Notes        string                       `json:"notes,omitempty"`
}

type inboxDestinationAssignment struct {
	Path          string             `json:"path"`
	ClaimIDs      []string           `json:"claim_ids,omitempty"`
	FinalMarkdown string             `json:"final_markdown,omitempty"`
	Ledger        []InboxClaimLedger `json:"ledger,omitempty"`
	Confidence    float64            `json:"confidence"`
	Reason        string             `json:"reason,omitempty"`
}

func inboxDecisionMessages(sourcePath string, sourceContent string, candidates []scoredCandidate, options Options, shorthandPrompt string) []provider.Message {
	payload := make([]map[string]any, 0, len(candidates))
	charLimit := inboxCandidateCharLimit(options, len(candidates))
	for i, candidate := range candidates {
		excerpt, _ := notetext.NumberedExcerpt(candidate.Path, candidate.Content, charLimit)
		payload = append(payload, map[string]any{
			"id":               i + 1,
			"path":             candidate.Path,
			"similarity":       candidate.Similarity,
			"numbered_excerpt": excerpt,
		})
	}
	user, _ := json.MarshalIndent(map[string]any{
		"source_path": sourcePath,
		"source_note": sourceContent,
		"candidates":  payload,
		"required_schema": map[string]any{
			"claims": []map[string]any{{
				"id":     "stable id like c1",
				"text":   "one atomic factual claim in English",
				"source": "short source quote or block reference",
			}},
			"destinations": []map[string]any{{
				"path":           "destination candidate path; must be one of candidates[].path",
				"claim_ids":      []string{"backward-compatible ids represented by this destination"},
				"confidence":     "number 0..1",
				"final_markdown": "complete destination note after edits; omit or empty when every claim is deduped/pending",
				"ledger": []map[string]any{{
					"claim_id":         "claim id",
					"status":           "merged, deduped, or pending",
					"destination_path": "same destination candidate path",
					"evidence":         "line(s) proving dedupe or exact inserted/edited wording for merge",
					"reason":           "short reason",
				}},
				"reason": "short reason",
			}},
			"pending": []map[string]any{{
				"claim_id": "claim id that cannot be safely routed",
				"status":   "pending",
				"reason":   "short reason",
			}},
			"validation": map[string]any{
				"verdict":               "pass or fail",
				"score":                 "number 0..1",
				"missing_facts":         []string{"facts missing from final markdown or dedupe ledger"},
				"unsupported_additions": []string{"facts added without support in source/candidate"},
				"notes":                 "short self-check",
			},
			"notes": "short explanation",
		},
	}, "", "  ")
	return []provider.Message{
		{Role: "system", Content: strings.Join([]string{
			"You perform the complete no-loss UPSC zettelkasten inbox merge in one JSON response.",
			"Return JSON only.",
			"Extract every factual claim, definition, date, statistic, proper noun, list item, qualifier, and analytical relation from source_note.",
			"Translate claims to English.",
			"Filter gossip, rumors, unsupported opinion, and political speculation.",
			"Do not summarize away details; split dense content into enough claims to preserve every detail.",
			"Use destinations only from the provided candidate paths.",
			"Allow split routing when one source note contains claims for multiple destinations.",
			"Use status=deduped only when the visible numbered candidate excerpt already represents the claim.",
			"Use status=merged only when final_markdown includes the minimal edit needed for that claim.",
			"Use status=pending when the candidate is truncated before the relevant location, confidence is low, the claim conflicts, or no candidate is a safe home.",
			"final_markdown must be the complete destination note after edits, not a patch, diff, summary, or explanation.",
			"If every claim for a destination is deduped or pending, omit final_markdown or leave it empty.",
			"Preserve existing destination wording, symbols, operators, numbers, qualifiers, and order unless a minimal edit is required.",
			"Do not make style-only rewrites to existing destination content.",
			"Do not add external knowledge.",
			"Set validation verdict=pass only if every non-pending source claim is represented by final_markdown or a deduped ledger entry, every existing visible destination fact remains represented, and there are no unsupported additions.",
			"Set validation verdict=fail if any merged/deduped claim is not safely represented.",
			"",
			"EXTREME SHORTHAND STYLE TEMPLATE:",
			shorthandPrompt,
		}, "\n")},
		{Role: "user", Content: string(user)},
	}
}

func inboxCandidateCharLimit(options Options, count int) int {
	if count < 1 {
		count = 1
	}
	perCandidate := options.MaxMergeInputChars / count
	if perCandidate < options.CandidateJudgeChars {
		return options.CandidateJudgeChars
	}
	return perCandidate
}
