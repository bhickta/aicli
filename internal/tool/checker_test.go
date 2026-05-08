package tool

import "testing"

func TestFirstLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "unix newline", in: "one\ntwo", want: "one"},
		{name: "carriage return", in: "one\rtwo", want: "one"},
		{name: "single line", in: "one", want: "one"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := firstLine(tt.in); got != tt.want {
				t.Fatalf("firstLine(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
