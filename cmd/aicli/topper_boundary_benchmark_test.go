package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bhickta/aicli/internal/workflow/analyze"
)

func TestRunTopperBoundaryBenchmarkIO(t *testing.T) {
	t.Parallel()

	var chatCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/models":
			writeBenchmarkTestJSON(t, w, map[string]any{
				"models": []any{
					map[string]any{
						"key":              "local/model",
						"display_name":     "Local Model",
						"loaded_instances": []any{map[string]any{"id": "local/model"}},
					},
				},
			})
		case "/v1/chat/completions":
			chatCalls.Add(1)
			var request struct {
				Model          string `json:"model"`
				ResponseFormat struct {
					Type       string `json:"type"`
					JSONSchema struct {
						Name   string `json:"name"`
						Strict bool   `json:"strict"`
					} `json:"json_schema"`
				} `json:"response_format"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatal(err)
			}
			if request.Model != "local/model" || request.ResponseFormat.Type != "json_schema" || request.ResponseFormat.JSONSchema.Name != "topper_answer_boundary_ledger" || !request.ResponseFormat.JSONSchema.Strict {
				t.Fatalf("chat request = %#v, want exact model and strict production schema", request)
			}
			content := `{"decisions":[{"page_number":1,"boundary":"new_answer","boundary_confidence":0.95,"visible_label":"Q1","label_evidence":"Q1","reason":"visible"},{"page_number":2,"boundary":"continuation","boundary_confidence":0.9,"visible_label":"","label_evidence":"","reason":"continues"}]}`
			writeBenchmarkTestJSON(t, w, map[string]any{
				"choices": []any{map[string]any{
					"message":       map[string]any{"role": "assistant", "content": content},
					"finish_reason": "stop",
				}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	inputPath := writeBoundaryBenchmarkTestSuite(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := runTopperBoundaryBenchmarkIO([]string{
		"-input", inputPath,
		"-model", "local/model",
		"-base-url", server.URL + "/v1",
		"-api-key", "test",
		"-runs", "2",
	}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %s", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if chatCalls.Load() != 2 {
		t.Fatalf("chat calls = %d, want 2", chatCalls.Load())
	}
	var report analyze.BoundaryBenchmarkReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not benchmark JSON: %v\n%s", err, stdout.String())
	}
	if report.Model != "local/model" || report.Summary.ExactGroupingRuns != 2 || report.Summary.LabelMatches != 2 {
		t.Fatalf("report = %#v, want two exact stable runs", report)
	}
}

func TestRunTopperBoundaryBenchmarkIORejectsUnloadedModel(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/models" {
			t.Fatalf("unexpected request path %q", r.URL.Path)
		}
		writeBenchmarkTestJSON(t, w, map[string]any{
			"models": []any{map[string]any{
				"key":              "other/model",
				"loaded_instances": []any{map[string]any{"id": "other/model"}},
			}},
		})
	}))
	t.Cleanup(server.Close)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := runTopperBoundaryBenchmarkIO([]string{
		"-input", writeBoundaryBenchmarkTestSuite(t),
		"-model", "missing/model",
		"-base-url", server.URL + "/v1",
	}, &stdout, &stderr)
	if exitCode != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "is not loaded") {
		t.Fatalf("exit=%d stdout=%q stderr=%q, want unloaded-model failure", exitCode, stdout.String(), stderr.String())
	}
}

func TestRunTopperBoundaryBenchmarkIORequiresInput(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if exitCode := runTopperBoundaryBenchmarkIO([]string{"-model", "local/model"}, &stdout, &stderr); exitCode != 2 {
		t.Fatalf("exit code = %d, want usage error 2", exitCode)
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "missing -input") {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func writeBoundaryBenchmarkTestSuite(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "suite.json")
	suite := `{"version":1,"cases":[{"id":"two-pages","pages":[{"number":1,"text":"Q1 answer"},{"number":2,"text":"continuation"}],"expected_questions":[{"label":"Q1","source_pages":[1,2],"status":"detected"}]}]}`
	if err := os.WriteFile(path, []byte(suite), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeBenchmarkTestJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatal(err)
	}
}
