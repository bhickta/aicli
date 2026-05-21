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
				"text":   "one coherent concept merge unit in English; keep related details, examples, and qualifiers together",
				"source": "short source heading, quote, or block reference",
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
					"evidence":         "destination section or inserted wording representing the concept unit",
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
			"Extract coherent concept units from source_note, not line-by-line atomic fragments.",
			"Prefer one unit per connected idea, heading, paragraph block, or example cluster.",
			"Keep related definitions, examples, qualifiers, root lists, and application lists together inside the same unit when they explain one concept.",
			"Translate claims to English.",
			"Filter gossip, rumors, unsupported opinion, and political speculation.",
			"Do not summarize away meaningful details, but avoid splitting a concept into many tiny ledger rows.",
			"Use destinations only from the provided candidate paths.",
			"Choose destinations by conceptual fit, not by exact keyword overlap. A note about the overall definition or nature of Economics can merge into an Economics etymology/definition note.",
			"Allow split routing only when the source contains clearly separate concepts that belong in different destination notes.",
			"Use status=deduped only when the visible numbered candidate excerpt already represents the concept unit.",
			"Use status=merged only when final_markdown includes the minimal edit needed for that concept unit.",
			"Use status=pending when no candidate is a conceptually safe home, the candidate is truncated before the relevant location, confidence is low, or the source conflicts with the destination.",
			"final_markdown must be the complete destination note after edits, not a patch, diff, summary, or explanation.",
			"If every concept unit for a destination is deduped or pending, omit final_markdown or leave it empty.",
			"Preserve existing destination wording, symbols, operators, numbers, qualifiers, and order unless a minimal edit is required.",
			"Do not make style-only rewrites to existing destination content.",
			"Do not add external knowledge.",
			"Set validation verdict=pass when every non-pending source concept unit is represented at concept level by final_markdown or a deduped ledger entry, every existing visible destination concept remains represented, and there are no unsupported additions.",
			"Set validation verdict=fail if any merged/deduped concept unit is not safely represented.",
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
