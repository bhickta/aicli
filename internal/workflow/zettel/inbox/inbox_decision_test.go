package inbox

import (
	"slices"
	"testing"
)

func TestDestinationRouteFitsRejectsBroadClaimForNarrowDestination(t *testing.T) {
	t.Parallel()

	destination := inboxDestinationAssignment{
		Path: "zettelkasten/Economy/UPSC Prelims Concepts on Debt and Deficit.md",
		Actions: []inboxDestinationAction{{
			ClaimID: "c1",
			Type:    "append_to_end",
			Lines:   []string{"- **UPSC Prelims Syllabus**: Poverty, inclusion, demographics, social-sector initiatives."},
		}},
	}
	claims := []InboxClaim{{
		ID:   "c1",
		Text: "UPSC Prelims syllabus includes poverty, inclusion, demographics, and social-sector initiatives.",
	}}
	item := InboxClaimLedger{ClaimID: "c1", Status: claimStatusMerged}

	if destinationRouteFits(destination.Path, destination, claims, item) {
		t.Fatal("destinationRouteFits() accepted broad syllabus claim for debt/deficit destination")
	}
}

func TestDestinationRouteFitsAllowsConceptuallyMatchingDestination(t *testing.T) {
	t.Parallel()

	destination := inboxDestinationAssignment{
		Path: "zettelkasten/Economy/Shubra Ranjan Economy/178_Sectoral_Economic_Dynamics_and_Jobless_Growth.md",
		Actions: []inboxDestinationAction{{
			ClaimID: "c1",
			Type:    "append_to_end",
			Lines:   []string{"- **UPSC 2022 Mains Trigger**: labor-productivity-led growth -> Jobless Growth."},
		}},
	}
	claims := []InboxClaim{{
		ID:   "c1",
		Text: "UPSC 2022 Mains statement about labor-productivity-led growth maps to Jobless Growth.",
	}}
	item := InboxClaimLedger{ClaimID: "c1", Status: claimStatusMerged}

	if !destinationRouteFits(destination.Path, destination, claims, item) {
		t.Fatal("destinationRouteFits() rejected matching Jobless Growth destination")
	}
}

func TestApplyDestinationActionIndentsChildBulletsUnderParentHeading(t *testing.T) {
	t.Parallel()

	lines := []string{"- **UPSC Prelims Concepts**:"}
	action := inboxDestinationAction{
		Type:       "insert_after_exact_line",
		Anchor:     "- **UPSC Prelims Concepts**:",
		LineNumber: flexibleLineNumber(1),
		Lines:      []string{"- **Prelims Syllabus**: Poverty and inclusion."},
	}

	got, changed, represented, reason := applyDestinationAction(lines, action)
	if reason != "" {
		t.Fatalf("applyDestinationAction() reason = %q, want success", reason)
	}
	if !changed || !represented {
		t.Fatalf("changed=%v represented=%v, want both true", changed, represented)
	}
	want := []string{
		"- **UPSC Prelims Concepts**:",
		"\t- **Prelims Syllabus**: Poverty and inclusion.",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("lines = %#v, want %#v", got, want)
	}
}

func TestDestinationContainsClaimUsesTokenCoverageForDedupe(t *testing.T) {
	t.Parallel()

	claims := []InboxClaim{{
		ID:   "c1",
		Text: "Rote learning fails in UPSC",
	}}
	item := InboxClaimLedger{ClaimID: "c1", Status: claimStatusDeduped}
	content := "- **UPSC**: Rote learning fails; independent thinking needed.\n"

	if !destinationContainsClaim(content, claims, item) {
		t.Fatal("destinationContainsClaim() rejected mechanically represented dedupe claim")
	}
	if destinationContainsClaim("- **UPSC**: Independent thinking needed.\n", claims, item) {
		t.Fatal("destinationContainsClaim() accepted missing dedupe claim")
	}
}
