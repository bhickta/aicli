package indexer

import "testing"

func TestCosineSimilarity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a    []float64
		b    []float64
		want float64
	}{
		{name: "same direction", a: []float64{1, 0}, b: []float64{1, 0}, want: 1},
		{name: "orthogonal", a: []float64{1, 0}, b: []float64{0, 1}, want: 0},
		{name: "mismatched dimensions", a: []float64{1}, b: []float64{1, 0}, want: 0},
		{name: "zero vector", a: []float64{0, 0}, b: []float64{1, 0}, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := cosineSimilarity(tt.a, tt.b); got != tt.want {
				t.Fatalf("cosineSimilarity() = %v, want %v", got, tt.want)
			}
		})
	}
}
