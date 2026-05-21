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

func TestFinalNotesValidationFindsMissingSourceLine(t *testing.T) {
	t.Parallel()

	source := "- **Fiscal Policy**: Government Budget.\n- **Monetary Policy**: RBI money control.\n"
	finalNotes := map[string]string{"zettelkasten/budget.md": "- **Fiscal Policy**: Government Budget.\n"}

	judge := finalNotesValidation(source, finalNotes)
	if judge.Verdict != "fail" || len(judge.MissingFacts) != 1 {
		t.Fatalf("judge = %#v, want missing monetary policy line", judge)
	}
}

func TestFinalNotesValidationPassesCoveredSource(t *testing.T) {
	t.Parallel()

	source := "- **Fiscal Policy**: Government Budget.\n- **Monetary Policy**: RBI money control.\n"
	finalNotes := map[string]string{"zettelkasten/tools.md": "- **Macro Tools**: Fiscal Policy = Government Budget; Monetary Policy = RBI money control.\n"}

	judge := finalNotesValidation(source, finalNotes)
	if judge.Verdict != "pass" {
		t.Fatalf("judge = %#v, want pass", judge)
	}
}
