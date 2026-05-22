package inbox

import (
	"strings"
	"testing"
)

func TestInboxMergePromptRequiresPassthroughForNewDestination(t *testing.T) {
	t.Parallel()

	messages := inboxMergeMessages(
		"in/topic.md",
		"---\nStatus: Read\n---\n- **Exact Quote**: \"Do not paraphrase\".\n",
		[]scoredCandidate{{Path: "zettelkasten/topic.md"}},
		Options{},
		"",
	)

	system := messages[0].Content
	user := messages[1].Content
	if !strings.Contains(user, newDestinationPassthroughMarker) {
		t.Fatalf("user prompt missing passthrough marker: %s", user)
	}
	if !strings.Contains(system, "output the SOURCE NOTE exactly") {
		t.Fatalf("system prompt missing exact passthrough instruction: %s", system)
	}
	if !strings.Contains(system, "do not paraphrase") || !strings.Contains(system, "collapse lists") {
		t.Fatalf("system prompt missing rewrite prohibition: %s", system)
	}
}

func TestInboxValidationPromptRejectsNewDestinationRewrites(t *testing.T) {
	t.Parallel()

	messages := inboxValidationMessages(
		"in/topic.md",
		"- **Roadmap**:\n\t1. Poverty.\n\t2. Microeconomics.\n",
		[]scoredCandidate{{Path: "zettelkasten/topic.md"}},
		inboxDestinationDecision{
			Destinations: []inboxDestinationAssignment{{
				Path:      "zettelkasten/topic.md",
				FinalNote: "- **Roadmap**: 1. Poverty. 2. Microeconomics.\n",
			}},
		},
	)

	system := messages[0].Content
	user := messages[1].Content
	if !strings.Contains(user, newDestinationPassthroughMarker) {
		t.Fatalf("user prompt missing passthrough marker: %s", user)
	}
	if !strings.Contains(system, "PASS only if the proposed final note preserves the SOURCE NOTE exactly") {
		t.Fatalf("system prompt missing strict new-note validation: %s", system)
	}
	if !strings.Contains(system, "collapsed into one long line") {
		t.Fatalf("system prompt missing line-collapse rejection: %s", system)
	}
}
