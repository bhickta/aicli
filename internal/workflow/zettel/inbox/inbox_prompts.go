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
}

type inboxDestinationAssignment struct {
	Path       string   `json:"path"`
	ClaimIDs   []string `json:"claim_ids,omitempty"`
	FinalNote  string   `json:"final_note,omitempty"`
	Confidence float64  `json:"confidence"`
	Reason     string   `json:"reason,omitempty"`
}

func inboxMergeMessages(sourcePath string, sourceContent string, candidates []scoredCandidate, options Options, shorthandPrompt string) []provider.Message {
	user := strings.Join([]string{
		"SOURCE PATH:",
		sourcePath,
		"",
		"SOURCE NOTE:",
		sourceContent,
		"",
		"SEMANTIC DESTINATION CANDIDATES:",
		formatInboxCandidateNotes(candidates, options),
	}, "\n")
	return []provider.Message{
		{Role: "system", Content: strings.Join([]string{
			"You perform a complete no-loss UPSC zettelkasten inbox merge using only the provided semantic destination candidates.",
			"Return final destination notes only. Do not return JSON. Do not return line numbers. Do not return patch actions.",
			"Use this exact plain-text envelope for each destination you want rewritten:",
			"BEGIN_NOTE <candidate path>",
			"<complete final markdown for that note>",
			"END_NOTE",
			"If no destination is safe, return: PENDING: <short reason>.",
			"Each BEGIN_NOTE path must be one of the provided candidate paths.",
			"Inside each note block, write the whole final note, not only the new lines.",
			"Keep existing YAML frontmatter exactly.",
			"Choose the smallest number of candidate notes that can preserve the source facts without breaking atomicity.",
			"Prefer one destination when the source is one conceptual note.",
			"Use multiple destinations only when the source clearly contains separate atomic concepts.",
			"Use plain readable labels: `- **Concept Label**: fact`. Never use `::`, snake_case, underscore-separated headings, or vague abbreviations.",
			"Do not create generic labels such as `**Label**`, `**Topic**`, `**Heading**`, `**Title**`, or `**Note**`; write the real concept label.",
			"Every final note must be a complete deduplicated superset of the original destination note plus assigned source facts.",
			"You may rewrite the whole note, but no useful existing destination fact may disappear.",
			"Preserve every existing destination fact unless it is already represented by an equivalent final fact.",
			"Preserve the destination note's existing markdown hierarchy; do not flatten existing nested bullets into top-level bullets.",
			"Add related source facts at the matching level under the relevant parent bullet or section when one already exists.",
			"Preserve every useful source fact, number, qualifier, example, definition, and relationship.",
			"If the source repeats an existing destination fact, merge the new wording/details into the existing bullet instead of adding a duplicate bullet.",
			"Avoid repeating the same heading or concept label unless each repeated bullet adds a clearly different fact.",
			"Concept labels must name one real concept from the source or destination; do not combine unrelated abbreviations, exam references, examples, or adjacent labels into a new label.",
			"When a source concept only loosely fits the destination, keep it as a separate clearly labeled section inside the closest candidate.",
			"Preserve quoted phrases exactly, including every word inside quotation marks.",
			"Preserve contextual numbers exactly: years, percentages, rupee amounts, star ratings, marks, question references like Q1/Q2, hours, and stated counts.",
			"For legal, regulatory, policy, court, or dispute chains, preserve the sequence exactly: actor, forum/body, decision, who challenged it, final outcome, and who benefited.",
			"Do not replace specific outcome verbs like stayed, set aside, rejected, upheld, allowed, or disallowed with a different verb unless the source uses or clearly supports that exact outcome.",
			"Markdown list numbering such as `1.` or `2.` is formatting, not a fact, unless the number is part of the concept label.",
			"Never end any markdown line with two spaces; use normal newlines only.",
			"Merge conceptually related source facts into the most appropriate final note.",
			"Do not merge a broad syllabus, roadmap, strategy, or overview note into a narrow destination about one subtopic.",
			"Use a candidate only when the final note remains atomic and coherent after the merge.",
			"Do not create or name a new destination note.",
			"Filter gossip, rumors, unsupported opinion, and political speculation.",
			"Do not add external knowledge.",
			"",
			"EXTREME SHORTHAND STYLE TEMPLATE:",
			shorthandPrompt,
		}, "\n")},
		{Role: "user", Content: string(user)},
	}
}

func formatInboxCandidateNotes(candidates []scoredCandidate, options Options) string {
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
	return strings.Join(candidateText, "\n\n")
}

func inboxCandidateCharLimit(options Options, count int) int {
	const minCandidateExcerptChars = 2500

	if count < 1 {
		count = 1
	}
	perCandidate := options.MaxMergeInputChars / count
	if perCandidate < minCandidateExcerptChars {
		return minCandidateExcerptChars
	}
	return perCandidate
}
