package inbox

import (
	"encoding/json"
	"strings"

	"github.com/bhickta/aicli/internal/provider"
)

type inboxDestinationDecision struct {
	Claims       []InboxClaim                 `json:"claims,omitempty"`
	Destinations []inboxDestinationAssignment `json:"destinations"`
	Pending      []InboxClaimLedger           `json:"pending"`
	Notes        string                       `json:"notes,omitempty"`
}

type inboxDestinationAssignment struct {
	Path       string             `json:"path"`
	ClaimIDs   []string           `json:"claim_ids"`
	Ledger     []InboxClaimLedger `json:"ledger,omitempty"`
	Confidence float64            `json:"confidence"`
	Reason     string             `json:"reason,omitempty"`
}

type inboxRewritePlan struct {
	FinalMarkdown string             `json:"final_markdown"`
	Ledger        []InboxClaimLedger `json:"ledger"`
	Notes         string             `json:"notes,omitempty"`
}

func inboxRouteMessages(sourcePath string, sourceContent string, candidates []scoredCandidate, options Options) []provider.Message {
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
				"path":       "destination candidate path",
				"claim_ids":  []string{"backward-compatible claim ids needing a merge rewrite"},
				"confidence": "number 0..1",
				"ledger": []map[string]any{{
					"claim_id":         "claim id",
					"status":           "merged or deduped",
					"destination_path": "same destination candidate path",
					"evidence":         "numbered excerpt line(s) proving dedupe, or why rewrite is needed",
					"reason":           "short reason",
				}},
				"reason": "short reason",
			}},
			"pending": []map[string]any{{
				"claim_id": "claim id that cannot be safely routed",
				"status":   "pending",
				"reason":   "short reason",
			}},
			"notes": "short explanation",
		},
	}, "", "  ")
	return []provider.Message{
		{Role: "system", Content: strings.Join([]string{
			"You extract atomic claims from a source note and route them into existing UPSC zettelkasten destination notes.",
			"Return JSON only.",
			"Extract every factual claim, definition, date, statistic, proper noun, list item, qualifier, and analytical relation.",
			"Translate claims to English.",
			"Filter gossip, rumors, unsupported opinion, and political speculation.",
			"Do not summarize away details; split dense content into enough claims to preserve every detail.",
			"Use destinations only from the provided candidate paths.",
			"Allow split routing when one source note contains claims for multiple destinations.",
			"Only assign a claim when confidence is high that it belongs in that destination.",
			"Inside each destination ledger, use status=deduped only when the numbered excerpt already represents the claim.",
			"Use status=merged only when the destination is the correct home but needs an edit.",
			"Do not send already-deduped claims to rewrite; mark them deduped in the routing ledger.",
			"Mark uncertain claims as pending.",
		}, "\n")},
		{Role: "user", Content: string(user)},
	}
}

func inboxRewriteMessages(destinationPath string, destinationContent string, sourcePath string, claims []InboxClaim, shorthandPrompt string) []provider.Message {
	payload, _ := json.MarshalIndent(map[string]any{
		"destination_path": destinationPath,
		"destination_note": destinationContent,
		"source_path":      sourcePath,
		"claims":           claims,
		"required_schema": map[string]any{
			"final_markdown": "destination note with only necessary claim insertions/edits applied",
			"ledger": []map[string]any{{
				"claim_id":         "claim id",
				"status":           "merged, deduped, or pending",
				"destination_path": destinationPath,
				"evidence":         "where/how represented, or judge dedupe statement",
				"reason":           "short reason",
			}},
			"notes": "short explanation",
		},
	}, "", "  ")
	return []provider.Message{
		{Role: "system", Content: strings.Join([]string{
			"You merge source claims into one UPSC destination note.",
			"Return JSON only.",
			"Preserve existing destination wording, symbols, operators, numbers, qualifiers, and order unless a minimal edit is required to insert a merged source claim.",
			"Do not make style-only rewrites to existing destination content.",
			"Only change final_markdown when at least one claim is status=merged.",
			"If every claim is deduped or pending, final_markdown must be byte-for-byte identical to destination_note.",
			"Preserve every existing destination fact and every source claim marked merged.",
			"Use status=deduped only when the claim is already represented in the destination note.",
			"Use status=pending when the claim cannot be safely merged or deduped.",
			"Do not add external knowledge.",
			"",
			"EXTREME SHORTHAND STYLE TEMPLATE:",
			shorthandPrompt,
		}, "\n")},
		{Role: "user", Content: string(payload)},
	}
}

func inboxValidationMessages(sourcePath string, sourceContent string, destinationBefore map[string]string, destinationAfter map[string]string, ledger []InboxClaimLedger) []provider.Message {
	payload, _ := json.MarshalIndent(map[string]any{
		"source_path":        sourcePath,
		"source_note":        sourceContent,
		"destination_before": destinationBefore,
		"destination_after":  destinationAfter,
		"claim_ledger":       ledger,
		"required_schema": map[string]any{
			"verdict":               "pass or fail",
			"score":                 "number 0..1",
			"missing_facts":         []string{"facts missing from destination_after or ledger"},
			"unsupported_additions": []string{"facts in destination_after unsupported by source or destination_before"},
			"notes":                 "short explanation",
		},
	}, "", "  ")
	return []provider.Message{
		{Role: "system", Content: strings.Join([]string{
			"You are a strict no-loss UPSC shorthand merge verifier.",
			"Return JSON only.",
			"Pass only if every source factual claim is either present in destination_after or marked deduped in the ledger.",
			"Pass only if every existing destination_before fact remains represented in destination_after.",
			"Fail on unsupported external knowledge.",
		}, "\n")},
		{Role: "user", Content: string(payload)},
	}
}
