package inbox

import (
	"encoding/json"
	"fmt"
	"strconv"
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
	FinalNotes   bool                         `json:"-"`
}

type inboxDestinationAssignment struct {
	Path       string                   `json:"path"`
	ClaimIDs   []string                 `json:"claim_ids,omitempty"`
	Actions    []inboxDestinationAction `json:"actions,omitempty"`
	Ledger     []InboxClaimLedger       `json:"ledger,omitempty"`
	FinalNote  string                   `json:"final_note,omitempty"`
	Confidence float64                  `json:"confidence"`
	Reason     string                   `json:"reason,omitempty"`
}

type inboxDestinationAction struct {
	ClaimID    string             `json:"claim_id,omitempty"`
	Type       string             `json:"type"`
	Anchor     string             `json:"anchor,omitempty"`
	LineNumber flexibleLineNumber `json:"line_number,omitempty"`
	Line       string             `json:"line,omitempty"`
	Lines      []string           `json:"lines,omitempty"`
	Reason     string             `json:"reason,omitempty"`
}

type flexibleLineNumber int

func (n *flexibleLineNumber) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	if raw == "" || raw == "null" {
		*n = 0
		return nil
	}
	if strings.HasPrefix(raw, `"`) {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		*n = flexibleLineNumber(parseFlexibleLineNumber(value))
		return nil
	}
	*n = flexibleLineNumber(parseFlexibleLineNumber(raw))
	return nil
}

func parseFlexibleLineNumber(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
		return parsed
	}
	if parsed, err := strconv.ParseFloat(value, 64); err == nil && parsed > 0 {
		return int(parsed)
	}
	return 0
}

func inboxDecisionMessages(sourcePath string, sourceContent string, candidates []scoredCandidate, options Options, shorthandPrompt string) []provider.Message {
	var candidateText []string
	charLimit := inboxCandidateCharLimit(options, len(candidates))
	for i, candidate := range candidates {
		excerpt, _ := notetext.NumberedExcerpt(candidate.Path, candidate.Content, charLimit)
		candidateText = append(candidateText, strings.Join([]string{
			fmt.Sprintf("CANDIDATE %d", i+1),
			"PATH: " + candidate.Path,
			fmt.Sprintf("SIMILARITY: %.4f", candidate.Similarity),
			"CURRENT NOTE:",
			excerpt,
		}, "\n"))
	}
	user := strings.Join([]string{
		"SOURCE PATH:",
		sourcePath,
		"",
		"SOURCE NOTE:",
		sourceContent,
		"",
		"CANDIDATE DESTINATION NOTES:",
		strings.Join(candidateText, "\n\n"),
	}, "\n")
	return []provider.Message{
		{Role: "system", Content: strings.Join([]string{
			"You perform a complete no-loss UPSC zettelkasten inbox merge.",
			"Return final destination notes only. Do not return JSON. Do not return line numbers. Do not return patch actions.",
			"Use this exact plain-text envelope for each destination you want rewritten:",
			"BEGIN_NOTE <candidate path>",
			"<complete final markdown for that note>",
			"END_NOTE",
			"If no destination is safe, return: PENDING: <short reason>.",
			"Each BEGIN_NOTE path must be one of the provided candidate paths.",
			"Inside each note block, write the whole final note, not only the new lines.",
			"Keep existing YAML frontmatter exactly. For a new note, include YAML frontmatter with Status: Read.",
			"Use plain readable labels: `- **Label**: fact`. Never use `::`, snake_case, or underscore-separated headings.",
			"Preserve every existing destination fact unless it is an exact duplicate of the source merge.",
			"Preserve every useful source fact, number, qualifier, example, definition, and relationship.",
			"Merge conceptually related source facts into the most appropriate final note.",
			"Do not merge a broad syllabus, roadmap, strategy, or overview note into a narrow destination about one subtopic.",
			"Use a candidate only when the final note remains atomic and coherent after the merge.",
			"Filter gossip, rumors, unsupported opinion, and political speculation.",
			"Do not add external knowledge.",
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
