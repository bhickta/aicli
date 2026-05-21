package inbox

import "testing"

func TestMergeJudgePassedRequiresNoMissingFactsOrUnsupportedAdditions(t *testing.T) {
	t.Parallel()

	if !mergeJudgePassed(MergeJudge{Verdict: "pass", Score: 1}, 0.98) {
		t.Fatal("mergeJudgePassed() rejected clean passing judge")
	}
	if mergeJudgePassed(MergeJudge{Verdict: "pass", Score: 1, MissingFacts: []string{"missing"}}, 0.98) {
		t.Fatal("mergeJudgePassed() accepted judge with missing facts")
	}
	if mergeJudgePassed(MergeJudge{Verdict: "pass", Score: 1, UnsupportedAdditions: []string{"extra"}}, 0.98) {
		t.Fatal("mergeJudgePassed() accepted judge with unsupported additions")
	}
}
