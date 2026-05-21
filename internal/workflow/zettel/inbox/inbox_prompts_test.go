package inbox

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestInboxDestinationActionLineNumberAcceptsModelVariants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want int
	}{
		{name: "number", raw: `{"line_number":3}`, want: 3},
		{name: "numeric string", raw: `{"line_number":"3"}`, want: 3},
		{name: "empty string", raw: `{"line_number":""}`, want: 0},
		{name: "invalid string", raw: `{"line_number":"line 3"}`, want: 0},
		{name: "null", raw: `{"line_number":null}`, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var action inboxDestinationAction
			if err := json.Unmarshal([]byte(tt.raw), &action); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			if got := int(action.LineNumber); got != tt.want {
				t.Fatalf("line number = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestApplyDestinationActionFallsBackToAnchorWhenLineNumberIsInvalidString(t *testing.T) {
	t.Parallel()

	var action inboxDestinationAction
	raw := `{"type":"insert_after_exact_line","anchor":"- **Roadmap**: Existing.","line_number":"line 1","lines":["- **Next**: New."]}`
	if err := json.Unmarshal([]byte(raw), &action); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	got, changed, represented, reason := applyDestinationAction([]string{"- **Roadmap**: Existing."}, action)
	if reason != "" {
		t.Fatalf("applyDestinationAction() reason = %q, want success", reason)
	}
	if !changed || !represented {
		t.Fatalf("changed=%v represented=%v, want both true", changed, represented)
	}
	want := []string{"- **Roadmap**: Existing.", "- **Next**: New."}
	if !slices.Equal(got, want) {
		t.Fatalf("lines = %#v, want %#v", got, want)
	}
}
