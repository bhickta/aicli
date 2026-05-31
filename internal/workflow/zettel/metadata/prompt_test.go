package metadata

import (
	"strings"
	"testing"
)

func TestParseGeneratedMetadataAcceptsOneTelegraphicPrompt(t *testing.T) {
	t.Parallel()

	item, err := parseGeneratedMetadata(`{
		"title": "Chalcolithic Art, Craft, and Village Evolution",
		"summary_keywords": "Chalcolithic, art/culture, jewelry, beads, village evolution",
		"recall_questions": ["Chalcolithic art/culture -> jewelry + beads + village evolution"]
	}`)
	if err != nil {
		t.Fatalf("parseGeneratedMetadata() error = %v", err)
	}
	if len(item.RecallQuestions) != 1 {
		t.Fatalf("recall prompts = %#v, want one prompt", item.RecallQuestions)
	}
}

func TestParseGeneratedMetadataCapsRecallPromptsAtThree(t *testing.T) {
	t.Parallel()

	item, err := parseGeneratedMetadata(`{
		"title": "Economy Growth and Inflation",
		"summary_keywords": "growth, inflation, GDP, jobs",
		"recall_questions": [
			"Growth -> GDP",
			"Inflation -> prices",
			"Jobs -> employment",
			"Extra -> ignored"
		]
	}`)
	if err != nil {
		t.Fatalf("parseGeneratedMetadata() error = %v", err)
	}
	if len(item.RecallQuestions) != 3 {
		t.Fatalf("recall prompts = %#v, want capped at three", item.RecallQuestions)
	}
	if item.RecallQuestions[2] != "Jobs -> employment" {
		t.Fatalf("third prompt = %q, want third original prompt", item.RecallQuestions[2])
	}
}

func TestParseGeneratedMetadataRejectsQuestionAndMetaWording(t *testing.T) {
	t.Parallel()

	_, err := parseGeneratedMetadata(`{
		"title": "Chalcolithic Art and Culture",
		"summary_keywords": "Chalcolithic, art, culture",
		"recall_questions": ["What characteristics define the note's description of art and culture?"]
	}`)
	if err == nil {
		t.Fatal("parseGeneratedMetadata() error = nil, want validation error")
	}
	if !strings.Contains(err.Error(), "recall prompt") {
		t.Fatalf("error = %v, want recall prompt validation", err)
	}
}

func TestMetadataMessagesUseBodyOnly(t *testing.T) {
	t.Parallel()

	messages := metadataMessages("zettelkasten/course/source.md", "# Body\nActual facts.\n")
	if len(messages) != 2 {
		t.Fatalf("messages = %#v, want system and user messages", messages)
	}
	user := messages[1].Content
	for _, forbidden := range []string{"NOTE PATH", "source.md", "zettelkasten/course"} {
		if strings.Contains(user, forbidden) {
			t.Fatalf("user prompt leaked %q:\n%s", forbidden, user)
		}
	}
	if !strings.Contains(user, "# Body\nActual facts.") {
		t.Fatalf("user prompt missing body:\n%s", user)
	}
}
