package zettel

import (
	"encoding/json"
	"strings"

	"github.com/bhickta/aicli/internal/provider"
)

type inboxClaimExtraction struct {
	Claims []InboxClaim `json:"claims"`
	Notes  string       `json:"notes,omitempty"`
}

type inboxDestinationDecision struct {
	Destinations []inboxDestinationAssignment `json:"destinations"`
	Pending      []InboxClaimLedger           `json:"pending"`
	Notes        string                       `json:"notes,omitempty"`
}

type inboxDestinationAssignment struct {
	Path       string   `json:"path"`
	ClaimIDs   []string `json:"claim_ids"`
	Confidence float64  `json:"confidence"`
	Reason     string   `json:"reason,omitempty"`
}

type inboxRewritePlan struct {
	FinalMarkdown string             `json:"final_markdown"`
	Ledger        []InboxClaimLedger `json:"ledger"`
	Notes         string             `json:"notes,omitempty"`
}

func claimExtractionMessages(sourcePath string, sourceContent string) []provider.Message {
	payload, _ := json.MarshalIndent(map[string]any{
		"source_path": sourcePath,
		"source_note": sourceContent,
		"required_schema": map[string]any{
			"claims": []map[string]any{{
				"id":     "stable id like c1",
				"text":   "one atomic factual claim in English",
				"source": "short source quote or block reference",
			}},
			"notes": "short explanation",
		},
	}, "", "  ")
	return []provider.Message{
		{Role: "system", Content: strings.Join([]string{
			"You extract atomic UPSC study claims from Markdown.",
			"Return JSON only.",
			"Extract every factual claim, definition, date, statistic, proper noun, list item, qualifier, and analytical relation.",
			"Translate claims to English.",
			"Filter gossip, rumors, unsupported opinion, and political speculation.",
			"Do not summarize away details; split dense content into enough claims to preserve every detail.",
		}, "\n")},
		{Role: "user", Content: string(payload)},
	}
}

func inboxDestinationMessages(sourcePath string, claims []InboxClaim, candidates []scoredCandidate, options Options) []provider.Message {
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
		"claims":      claims,
		"candidates":  payload,
		"required_schema": map[string]any{
			"destinations": []map[string]any{{
				"path":       "destination candidate path",
				"claim_ids":  []string{"claim ids to merge or dedupe in this destination"},
				"confidence": "number 0..1",
				"reason":     "short reason",
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
			"You route source claims into existing UPSC zettelkasten destination notes.",
			"Return JSON only.",
			"Use destinations only from the provided candidate paths.",
			"Allow split routing when one source note contains claims for multiple destinations.",
			"Only assign a claim when confidence is high that it belongs in that destination.",
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
			"final_markdown": "entire destination note rewritten in English extreme shorthand",
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
			"Rewrite the entire destination note in English extreme shorthand using the style rules below.",
			"Preserve every existing destination fact and every source claim marked merged or deduped.",
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
