package inbox

import (
	"fmt"
	"strings"

	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/workflow/zettel/notetext"
)

type inboxDestinationDecision struct {
	Claims       []InboxClaim                 `json:"claims,omitempty"`
	Destinations []inboxDestinationAssignment `json:"destinations"`
	Pending      []InboxClaimLedger           `json:"pending"`
	Validation   MergeJudge                   `json:"validation,omitempty"`
}

type inboxDestinationAssignment struct {
	Path       string   `json:"path"`
	ClaimIDs   []string `json:"claim_ids,omitempty"`
	FinalNote  string   `json:"final_note,omitempty"`
	Confidence float64  `json:"confidence"`
	Reason     string   `json:"reason,omitempty"`
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
