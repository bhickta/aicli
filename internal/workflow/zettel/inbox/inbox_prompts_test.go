package inbox

import (
	"strings"
	"testing"
)

func TestInboxMergePromptUsesOnlySemanticCandidates(t *testing.T) {
	t.Parallel()

	messages := inboxMergeMessages(
		"in/topic.md",
		"- **Inflation**: 7% due to oil prices.\n",
		[]scoredCandidate{{
			Path:       "zettelkasten/economy.md",
			Content:    "- **Inflation**: 6%.\n",
			Similarity: 0.91,
		}},
		Options{},
		"",
	)

	system := messages[0].Content
	user := messages[1].Content
	if !strings.Contains(system, "using only the provided semantic destination candidates") {
		t.Fatalf("system prompt does not restrict merge to semantic candidates: %s", system)
	}
	if !strings.Contains(system, "Do not create or name a new destination note") {
		t.Fatalf("system prompt allows new destination notes: %s", system)
	}
	if strings.Contains(system, "adopted") || strings.Contains(user, "NEW EMPTY DESTINATION") {
		t.Fatalf("prompt still exposes adopted new-note flow:\n%s\n%s", system, user)
	}
	requiredInstructions := []string{
		"complete deduplicated superset",
		"no useful existing destination fact may disappear",
		"merge the new wording/details into the existing bullet instead of adding a duplicate bullet",
		"separate clearly labeled section inside the closest candidate",
		"Never end any markdown line with two spaces",
	}
	for _, instruction := range requiredInstructions {
		if !strings.Contains(system, instruction) {
			t.Fatalf("system prompt missing instruction %q: %s", instruction, system)
		}
	}
	if !strings.Contains(user, "SEMANTIC DESTINATION CANDIDATES:") || !strings.Contains(user, "zettelkasten/economy.md") {
		t.Fatalf("user prompt missing semantic candidates: %s", user)
	}
}
