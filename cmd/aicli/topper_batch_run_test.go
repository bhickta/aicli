package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/bhickta/aicli/internal/storage"
	"github.com/bhickta/aicli/internal/workflow/analyze"
)

func TestRunTopperBatchIO(t *testing.T) {
	t.Parallel()

	manifestPath := writeTopperBatchTestManifest(t, []string{"3663", "3664", "3665"})
	outputDir := filepath.Join(t.TempDir(), "batch-output")
	type submission struct {
		Lane                string
		OCRModel            string
		ReconciliationModel string
		Path                string
		ForceOCR            bool
		InputMode           string
	}
	var mu sync.Mutex
	jobs := map[string]storage.Job{}
	submissions := []submission{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/health":
			writeTopperBatchTestJSON(t, w, map[string]any{"status": "ok"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/providers":
			writeTopperBatchTestJSON(t, w, map[string]any{"providers": []map[string]any{
				{"id": "gemini-lane-2"},
				{"id": "unrelated"},
				{"id": "gemini-lane-1"},
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/workflows/analyze/run":
			var request struct {
				ProviderID    string `json:"provider_id"`
				OCRModel      string `json:"ocr_model"`
				BoundaryModel string `json:"boundary_model"`
				Path          string `json:"path"`
				ForceOCR      bool   `json:"force_ocr"`
				OCRInputMode  string `json:"ocr_input_mode"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatal(err)
			}
			mu.Lock()
			jobID := fmt.Sprintf("job-%d", len(submissions)+1)
			submissions = append(submissions, submission{
				Lane: request.ProviderID, OCRModel: request.OCRModel,
				ReconciliationModel: request.BoundaryModel, Path: request.Path,
				ForceOCR: request.ForceOCR, InputMode: request.OCRInputMode,
			})
			reviewJSON, err := json.Marshal(analyze.Response{
				Kind:      "topper_copy_review",
				ReviewID:  "review-" + filepath.Base(request.Path),
				PDFName:   filepath.Base(request.Path),
				Pages:     []analyze.Page{{Number: 1}},
				Questions: []analyze.Question{{ID: "Q1"}},
				APICalls:  12,
			})
			if err != nil {
				mu.Unlock()
				t.Fatal(err)
			}
			jobs[jobID] = storage.Job{ID: jobID, Status: storage.JobStatusCompleted, Output: string(reviewJSON)}
			mu.Unlock()
			writeTopperBatchTestJSON(t, w, map[string]any{"job": storage.Job{ID: jobID, Status: storage.JobStatusRunning}})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/jobs/"):
			mu.Lock()
			job, found := jobs[strings.TrimPrefix(r.URL.Path, "/api/jobs/")]
			mu.Unlock()
			if !found {
				http.NotFound(w, r)
				return
			}
			writeTopperBatchTestJSON(t, w, job)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := runTopperBatchIO([]string{
		"-manifest", manifestPath,
		"-output-dir", outputDir,
		"-server-url", server.URL,
		"-force-ocr",
		"-poll-interval", "1ms",
		"-timeout", "2s",
	}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %s", exitCode, stderr.String())
	}
	var result topperBatchResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout is not result JSON: %v\n%s", err, stdout.String())
	}
	if result.Total != 3 || result.Completed != 3 || result.Failed != 0 || len(result.Items) != 3 {
		t.Fatalf("result = %#v, want three completed copies", result)
	}
	for _, item := range result.Items {
		if item.ReviewID == "" || item.ReviewPath == "" {
			t.Fatalf("result item = %#v, want durable review identity and path", item)
		}
		data, err := os.ReadFile(item.ReviewPath)
		if err != nil {
			t.Fatalf("read persisted review: %v", err)
		}
		var review analyze.Response
		if err := json.Unmarshal(data, &review); err != nil {
			t.Fatalf("decode persisted review: %v", err)
		}
		if review.ReviewID != item.ReviewID {
			t.Fatalf("persisted review ID = %q, want %q", review.ReviewID, item.ReviewID)
		}
	}
	resultData, err := os.ReadFile(filepath.Join(outputDir, "result.json"))
	if err != nil {
		t.Fatalf("read persisted result: %v", err)
	}
	var persistedResult topperBatchResult
	if err := json.Unmarshal(resultData, &persistedResult); err != nil {
		t.Fatalf("decode persisted result: %v", err)
	}
	if persistedResult.Completed != result.Completed || len(persistedResult.Items) != len(result.Items) {
		t.Fatalf("persisted result = %#v, want %#v", persistedResult, result)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(submissions) != 3 {
		t.Fatalf("submissions = %d, want 3", len(submissions))
	}
	if submissions[0].Lane != "gemini-lane-1" || submissions[1].Lane != "gemini-lane-2" {
		t.Fatalf("initial lanes = %q, %q, want naturally sorted independent lanes", submissions[0].Lane, submissions[1].Lane)
	}
	for _, got := range submissions {
		if got.OCRModel != "gemini-flash-lite-latest" || got.ReconciliationModel != "gemini-3.5-flash" || !got.ForceOCR || got.InputMode != analyze.OCRInputModePDFDirect {
			t.Fatalf("submission = %#v, want hardened direct-PDF models and uncached run", got)
		}
	}
}

func TestRunTopperBatchIOReportsFailedJobs(t *testing.T) {
	t.Parallel()

	manifestPath := writeTopperBatchTestManifest(t, []string{"3670"})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/health":
			writeTopperBatchTestJSON(t, w, map[string]any{"status": "ok"})
		case r.URL.Path == "/api/providers":
			writeTopperBatchTestJSON(t, w, map[string]any{"providers": []map[string]any{{"id": "gemini-lane-1"}}})
		case r.Method == http.MethodPost:
			writeTopperBatchTestJSON(t, w, map[string]any{"job": storage.Job{ID: "failed-job", Status: storage.JobStatusRunning}})
		case r.URL.Path == "/api/jobs/failed-job":
			writeTopperBatchTestJSON(t, w, storage.Job{ID: "failed-job", Status: storage.JobStatusFailed, Error: "provider denied access"})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := runTopperBatchIO([]string{
		"-manifest", manifestPath,
		"-server-url", server.URL,
		"-poll-interval", "1ms",
		"-timeout", "2s",
	}, &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("exit code = %d, want runtime failure; stderr=%s", exitCode, stderr.String())
	}
	var result topperBatchResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Completed != 0 || result.Failed != 1 || result.Items[0].Error != "provider denied access" {
		t.Fatalf("result = %#v, want one evidence-bearing failed item", result)
	}
}

func TestWriteTopperBatchReviewIsImmutableAndIdempotent(t *testing.T) {
	t.Parallel()

	outputDir := t.TempDir()
	first := []byte(`{"kind":"topper_copy_review","review_id":"review-1"}`)
	path, err := writeTopperBatchReview(outputDir, "3663", first)
	if err != nil {
		t.Fatal(err)
	}
	reusedPath, err := writeTopperBatchReview(outputDir, "3663", first)
	if err != nil {
		t.Fatal(err)
	}
	if reusedPath != path {
		t.Fatalf("reused path = %q, want %q", reusedPath, path)
	}
	if _, err := writeTopperBatchReview(
		outputDir,
		"3663",
		[]byte(`{"kind":"topper_copy_review","review_id":"review-2"}`),
	); err == nil || !strings.Contains(err.Error(), "different review") {
		t.Fatalf("conflicting review error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(first) {
		t.Fatalf("immutable review changed to %q", data)
	}
}

func TestLoadTopperBatchManifestRejectsInvalidContracts(t *testing.T) {
	t.Parallel()

	pdf := filepath.Join(t.TempDir(), "copy.pdf")
	if err := os.WriteFile(pdf, []byte("%PDF-1.4\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		payload string
		want    string
	}{
		{
			name:    "unknown field",
			payload: fmt.Sprintf(`{"version":1,"copies":[{"source_id":"1","path":%q}],"extra":true}`, pdf),
			want:    "unknown field",
		},
		{
			name:    "duplicate source ID",
			payload: fmt.Sprintf(`{"version":1,"copies":[{"source_id":"1","path":%q},{"source_id":"1","path":%q}]}`, pdf, filepath.Join(t.TempDir(), "second.pdf")),
			want:    "duplicate source_id",
		},
		{
			name:    "unsupported version",
			payload: fmt.Sprintf(`{"version":2,"copies":[{"source_id":"1","path":%q}]}`, pdf),
			want:    "unsupported",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "manifest.json")
			if err := os.WriteFile(path, []byte(test.payload), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := loadTopperBatchManifest(path)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want text %q", err, test.want)
			}
		})
	}
}

func TestRunTopperBatchIORequiresManifest(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if exitCode := runTopperBatchIO(nil, &stdout, &stderr); exitCode != 2 {
		t.Fatalf("exit code = %d, want usage error 2", exitCode)
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "missing -manifest") {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func writeTopperBatchTestManifest(t *testing.T, sourceIDs []string) string {
	t.Helper()
	copies := make([]topperBatchCopy, 0, len(sourceIDs))
	for _, sourceID := range sourceIDs {
		path := filepath.Join(t.TempDir(), sourceID+".pdf")
		if err := os.WriteFile(path, []byte("%PDF-1.4\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		copies = append(copies, topperBatchCopy{SourceID: sourceID, Path: path})
	}
	manifestPath := filepath.Join(t.TempDir(), "manifest.json")
	data, err := json.Marshal(topperBatchManifest{Version: topperBatchManifestVersion, Copies: copies})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return manifestPath
}

func writeTopperBatchTestJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatal(err)
	}
}
