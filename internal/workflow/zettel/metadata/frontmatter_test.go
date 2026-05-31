package metadata

import "testing"

func TestHasSubstantiveBodySkipsEmptyAndHeadingOnlyNotes(t *testing.T) {
	t.Parallel()

	content := "---\nStatus: Read\n---\n# Title\n\n## Empty Section\n- **Heading Only**:\n"
	if hasSubstantiveBody(content) {
		t.Fatalf("hasSubstantiveBody(%q) = true, want false", content)
	}
}

func TestHasSubstantiveBodyAcceptsFactLines(t *testing.T) {
	t.Parallel()

	content := "---\nStatus: Read\n---\n# Title\n- **Bimbisara**: first ruler linked to Magadha expansion.\n"
	if !hasSubstantiveBody(content) {
		t.Fatalf("hasSubstantiveBody(%q) = false, want true", content)
	}
}

func TestYAMLQuoteKeepsReadableSymbols(t *testing.T) {
	t.Parallel()

	got := yamlQuote("GDP > inflation & jobs")
	want := `"GDP > inflation & jobs"`
	if got != want {
		t.Fatalf("yamlQuote() = %q, want %q", got, want)
	}
}
