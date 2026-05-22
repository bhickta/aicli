package inbox

import "testing"

func TestFinalNoteInboxValidationRejectsMissingSourceFact(t *testing.T) {
	t.Parallel()

	source := "- **Microeconomics**: \"Microscope\" view -> individual decisions.\n" +
		"- **Macroeconomics**: \"Telescope\" view -> country-level decisions and international relations.\n"
	applied := inboxAppliedDecision{
		destinationAfter: map[string]string{
			"zettelkasten/micro.md": "- **Microeconomics**: \"Microscope\" view -> individual decisions.\n",
		},
	}

	got := finalNoteInboxValidation(source, applied)
	if got.Verdict != "fail" || len(got.MissingFacts) != 1 {
		t.Fatalf("validation = %#v, want one missing macro fact", got)
	}
}

func TestFinalNoteInboxValidationRejectsUnsupportedCandidateContamination(t *testing.T) {
	t.Parallel()

	source := "- **Prelims Syllabus**: Poverty, inclusion, and demographics.\n"
	applied := inboxAppliedDecision{
		destinationBefore: map[string]string{
			"zettelkasten/prelims.md": "- **Microeconomics for Prelims**: Low priority.\n",
		},
		destinationAfter: map[string]string{
			"zettelkasten/prelims.md": "- **Microeconomics for Prelims**: Low priority.\n" +
				"- **Prelims Syllabus**: Poverty, inclusion, and demographics.\n" +
				"- **Mains Answer Writing Topics**: FDI in insurance and farm loan waiver.\n",
		},
	}

	got := finalNoteInboxValidation(source, applied)
	if got.Verdict != "fail" || len(got.UnsupportedAdditions) != 1 {
		t.Fatalf("validation = %#v, want one unsupported addition", got)
	}
}

func TestFinalNoteInboxValidationAllowsLineSupportedBySourceAndDestinationTogether(t *testing.T) {
	t.Parallel()

	source := "- **Microeconomics**: \"Microscope\" view -> individual decisions.\n"
	applied := inboxAppliedDecision{
		destinationBefore: map[string]string{
			"zettelkasten/micro.md": "- **Microeconomics**: Study of a particular firm or household.\n",
		},
		destinationAfter: map[string]string{
			"zettelkasten/micro.md": "- **Microeconomics**: \"Microscope\" view -> individual decisions. Study of a particular firm or household.\n",
		},
	}

	got := finalNoteInboxValidation(source, applied)
	if got.Verdict != "pass" {
		t.Fatalf("validation = %#v, want combined source+destination line accepted", got)
	}
}

func TestFinalNoteInboxValidationAcceptsMultiDestinationCoverage(t *testing.T) {
	t.Parallel()

	source := "- **Microeconomics**: \"Microscope\" view -> individual decisions.\n" +
		"- **Macroeconomics**: \"Telescope\" view -> country-level decisions and international relations.\n"
	applied := inboxAppliedDecision{
		destinationAfter: map[string]string{
			"zettelkasten/micro.md": "- **Microeconomics**: \"Microscope\" view -> individual decisions.\n",
			"zettelkasten/macro.md": "- **Macroeconomics**: \"Telescope\" view -> country-level decisions and international relations.\n",
		},
	}

	got := finalNoteInboxValidation(source, applied)
	if got.Verdict != "pass" || got.Score != 1 {
		t.Fatalf("validation = %#v, want pass", got)
	}
}

func TestConstrainFinalNoteRoutesRejectsNarrowDestination(t *testing.T) {
	t.Parallel()

	decision := inboxDestinationDecision{
		FinalNotes: true,
		Destinations: []inboxDestinationAssignment{{
			Path:      "zettelkasten/Economy/0330 Microeconomics Priority Note.md",
			ClaimIDs:  []string{finalNoteClaimID},
			FinalNote: "- **Prelims Syllabus**: Poverty and inclusion.\n",
		}},
	}
	source := "- **Prelims Syllabus**: Econ/Social Development, Poverty, Inclusion, Demographics.\n"

	got := constrainFinalNoteRoutes(decision, source)
	if len(got.Destinations) != 0 || len(got.Pending) != 1 {
		t.Fatalf("decision = %#v, want route rejected as pending", got)
	}
}
