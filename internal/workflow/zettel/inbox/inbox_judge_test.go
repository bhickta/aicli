package inbox

import "testing"

func TestParseInboxDestinationJudgementAcceptsTargetColon(t *testing.T) {
	t.Parallel()

	judgement, ok := parseInboxDestinationJudgement("TARGET: zettelkasten/Economy/note.md\n")
	if !ok || len(judgement.Targets) != 1 || judgement.Targets[0] != "zettelkasten/Economy/note.md" {
		t.Fatalf("parseInboxDestinationJudgement() = %#v, %v", judgement, ok)
	}
}

func TestParseInboxValidationResultAcceptsPassColon(t *testing.T) {
	t.Parallel()

	result := parseInboxValidationResult("PASS: all facts preserved\n")
	if !result.OK || !result.Pass {
		t.Fatalf("parseInboxValidationResult() = %#v, want pass", result)
	}
}
