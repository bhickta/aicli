package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadGeminiLaneKeys(t *testing.T) {
	tests := []struct {
		name        string
		payload     string
		mode        os.FileMode
		prepare     func(*testing.T)
		wantCount   int
		wantLaneOne string
		wantLaneTwo string
		wantError   string
	}{
		{
			name:        "loads valid records and skips unhealthy records",
			payload:     `[{"key":"first","status":"valid"},{"key":"bad","status":"unhealthy"},{"key":"second","status":"VALID"}]`,
			mode:        0o600,
			wantCount:   2,
			wantLaneOne: "first",
			wantLaneTwo: "second",
		},
		{
			name:    "existing environment is an explicit override",
			payload: `[{"key":"file-first","status":"valid"},{"key":"second","status":"valid"}]`,
			mode:    0o600,
			prepare: func(t *testing.T) {
				t.Setenv("GEMINI_LANE_1_KEY", "environment-first")
			},
			wantCount:   2,
			wantLaneOne: "environment-first",
			wantLaneTwo: "second",
		},
		{
			name:      "rejects duplicate valid keys",
			payload:   `[{"key":"same","status":"valid"},{"key":"same","status":"valid"}]`,
			mode:      0o600,
			wantError: "duplicate",
		},
		{
			name:      "rejects a valid key with surrounding whitespace",
			payload:   `[{"key":" secret ","status":"valid"}]`,
			mode:      0o600,
			wantError: "trimmed",
		},
		{
			name:      "rejects a file without valid keys",
			payload:   `[{"key":"bad","status":"unhealthy"}]`,
			mode:      0o600,
			wantError: "no valid keys",
		},
		{
			name:      "rejects group-readable secret files",
			payload:   `[{"key":"first","status":"valid"}]`,
			mode:      0o640,
			wantError: "permissions",
		},
		{
			name:      "rejects malformed JSON",
			payload:   `[{`,
			mode:      0o600,
			wantError: "parse key file JSON",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("GEMINI_LANE_1_KEY", "")
			t.Setenv("GEMINI_LANE_2_KEY", "")
			if test.prepare != nil {
				test.prepare(t)
			}
			path := filepath.Join(t.TempDir(), geminiKeysFilename)
			if err := os.WriteFile(path, []byte(test.payload), test.mode); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path, test.mode); err != nil {
				t.Fatal(err)
			}

			count, err := loadGeminiLaneKeys(path, false)
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("loadGeminiLaneKeys() error = %v, want %q", err, test.wantError)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if count != test.wantCount {
				t.Fatalf("loadGeminiLaneKeys() count = %d, want %d", count, test.wantCount)
			}
			if got := os.Getenv("GEMINI_LANE_1_KEY"); got != test.wantLaneOne {
				t.Fatalf("lane 1 = %q, want %q", got, test.wantLaneOne)
			}
			if got := os.Getenv("GEMINI_LANE_2_KEY"); got != test.wantLaneTwo {
				t.Fatalf("lane 2 = %q, want %q", got, test.wantLaneTwo)
			}
		})
	}
}

func TestLoadGeminiLaneKeysMissingOptionalFile(t *testing.T) {
	count, err := loadGeminiLaneKeys(filepath.Join(t.TempDir(), "missing.json"), true)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("count = %d, want 0", count)
	}
}

func TestLoadGeminiLaneKeysMissingRequiredFile(t *testing.T) {
	_, err := loadGeminiLaneKeys(filepath.Join(t.TempDir(), "missing.json"), false)
	if err == nil {
		t.Fatal("expected an error")
	}
}
