package inbox

import "testing"

func TestBadFinalNoteStyleAllowsExpandedTerms(t *testing.T) {
	t.Parallel()

	markdown := "- **Prelims Syllabus**: Sustainable Development and Social Sector Initiatives.\n"
	if reason := badFinalNoteStyle(markdown); reason != "" {
		t.Fatalf("badFinalNoteStyle() = %q, want expanded terms allowed", reason)
	}
}

func TestBadFinalNoteStyleRejectsAbbreviations(t *testing.T) {
	t.Parallel()

	markdown := "- **Prelims Syllabus**: Sust Dev, Social Dev, Social Sector Init, infinite dims.\n"
	if reason := badFinalNoteStyle(markdown); reason == "" {
		t.Fatal("badFinalNoteStyle() = empty, want shorthand rejection")
	}
}
