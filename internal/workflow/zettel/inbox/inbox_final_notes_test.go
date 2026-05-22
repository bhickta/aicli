package inbox

import "testing"

func TestParseInboxFinalNotes(t *testing.T) {
	t.Parallel()

	decision, ok := parseInboxFinalNotes("in/source.md", "BEGIN_NOTE zettelkasten/economics.md\n- final\nEND_NOTE\n")
	if !ok {
		t.Fatal("parseInboxFinalNotes() did not recognize final note envelope")
	}
	if !decision.FinalNotes || len(decision.Claims) != 1 || len(decision.Destinations) != 1 {
		t.Fatalf("decision = %#v, want one final-note destination and one source claim", decision)
	}
	if decision.Destinations[0].Path != "zettelkasten/economics.md" {
		t.Fatalf("path = %q", decision.Destinations[0].Path)
	}
	if decision.Destinations[0].FinalNote != "- final\n" {
		t.Fatalf("final note = %q", decision.Destinations[0].FinalNote)
	}
}

func TestFormatFinalNoteMarkdownPreservesExistingFrontmatterAndCleansLabels(t *testing.T) {
	t.Parallel()

	before := "---\nStatus: Read\nTags: economy\n---\n- **Old**: fact\n"
	markdown := "- **UPSC_Prelims_Economics_Syllabus**:: Econ/Social_Development.\n"

	got := formatFinalNoteMarkdown(before, markdown)
	want := "---\nStatus: Read\nTags: economy\n---\n- **UPSC Prelims Economics Syllabus**: Econ/Social Development."
	if got != want {
		t.Fatalf("formatted markdown = %q, want %q", got, want)
	}
}

func TestFormatFinalNoteMarkdownAddsDefaultFrontmatter(t *testing.T) {
	t.Parallel()

	got := formatFinalNoteMarkdown("", "- **Policy_Tools**:: Fiscal + Monetary.\n")
	want := "---\nStatus: Read\n---\n- **Policy Tools**: Fiscal + Monetary."
	if got != want {
		t.Fatalf("formatted markdown = %q, want %q", got, want)
	}
}

func TestFormatFinalNoteMarkdownReplacesChangedFrontmatter(t *testing.T) {
	t.Parallel()

	before := "---\nStatus: Read\nTags: economy\n---\n- **Old**: fact\n"
	markdown := "---\nStatus: Draft\n---\n- **New_Label**:: fact\n"

	got := formatFinalNoteMarkdown(before, markdown)
	want := "---\nStatus: Read\nTags: economy\n---\n- **New Label**: fact"
	if got != want {
		t.Fatalf("formatted markdown = %q, want %q", got, want)
	}
}
