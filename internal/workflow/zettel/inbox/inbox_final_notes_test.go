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
