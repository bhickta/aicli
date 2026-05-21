package manual

import "testing"

func TestNormalizeRangesRejectsOutOfBoundsLine(t *testing.T) {
	t.Parallel()

	_, err := normalizeRanges([]LineRange{{StartLine: 2, EndLine: 4}}, "one\ntwo", 2)
	if err == nil {
		t.Fatal("expected out-of-bounds range error")
	}
}
