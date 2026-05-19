package server

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/storage"
)

type memoryStore struct {
	jobs map[string]storage.Job
}

func (m *memoryStore) Migrate() error { return nil }
func (m *memoryStore) CreateJob(_ context.Context, job storage.Job) error {
	m.jobs[job.ID] = job
	return nil
}
func (m *memoryStore) GetJob(_ context.Context, id string) (storage.Job, error) {
	job, ok := m.jobs[id]
	if !ok {
		return storage.Job{}, storage.ErrNotFound
	}
	return job, nil
}
func (m *memoryStore) ListJobs(_ context.Context) ([]storage.Job, error) {
	jobs := make([]storage.Job, 0, len(m.jobs))
	for _, job := range m.jobs {
		jobs = append(jobs, job)
	}
	return jobs, nil
}
func (m *memoryStore) UpdateJob(_ context.Context, job storage.Job) error {
	m.jobs[job.ID] = job
	return nil
}

func TestHealth(t *testing.T) {
	t.Parallel()

	handler := testHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.Code)
	}
}

func TestCreateAndGetJob(t *testing.T) {
	t.Parallel()

	handler := testHandler()
	body := strings.NewReader(`{"id":"job-1","type":"ocr"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", body)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201, body=%s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/jobs/job-1", nil)
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("get status = %d, want 200", res.Code)
	}
	var job storage.Job
	if err := json.NewDecoder(res.Body).Decode(&job); err != nil {
		t.Fatal(err)
	}
	if job.Status != "queued" {
		t.Fatalf("Status = %q, want queued", job.Status)
	}
}

func TestListFiles(t *testing.T) {
	t.Parallel()

	handler := testHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/fs/list?path=.", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", res.Code, res.Body.String())
	}
}

func TestUploadFilesStoresDroppedFile(t *testing.T) {
	t.Parallel()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fileWriter, err := writer.CreateFormFile("file", "sample.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fileWriter.Write([]byte("%PDF-1.7\n")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	settings := config.DefaultSettings()
	settings.Tools = config.ToolConfig{}
	handler := testHandlerWithSettings(settings, t.TempDir())
	req := httptest.NewRequest(http.MethodPost, "/api/fs/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body=%s", res.Code, res.Body.String())
	}
	var payload struct {
		Files []uploadEntry `json:"files"`
		Root  string        `json:"root"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Files) != 1 {
		t.Fatalf("files len = %d, want 1", len(payload.Files))
	}
	if payload.Files[0].Name != "sample.pdf" {
		t.Fatalf("name = %q, want sample.pdf", payload.Files[0].Name)
	}
	if payload.Files[0].Path == "" {
		t.Fatal("path field is empty")
	}
	if payload.Files[0].URL == "" || payload.Files[0].URL == "/" {
		t.Fatalf("url field is invalid: %q", payload.Files[0].URL)
	}
	if !strings.HasPrefix(payload.Files[0].URL, "/uploads/") {
		t.Fatalf("url = %q, want /uploads/<file>", payload.Files[0].URL)
	}
	if _, err := os.Stat(payload.Files[0].Path); err != nil {
		t.Fatalf("uploaded file was not stored: %v", err)
	}

	getReq := httptest.NewRequest(http.MethodGet, payload.Files[0].URL, nil)
	getRes := httptest.NewRecorder()
	handler.ServeHTTP(getRes, getReq)
	if getRes.Code != http.StatusOK {
		t.Fatalf("get upload status = %d, want 200, body=%s", getRes.Code, getRes.Body.String())
	}
	if getRes.Body.String() != "%PDF-1.7\n" {
		t.Fatalf("uploaded file body = %q, want %q", getRes.Body.String(), "%PDF-1.7\n")
	}
}

func TestUploadFilesPreservesDroppedFolderShape(t *testing.T) {
	t.Parallel()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, name := range []string{"Course/01.mp4", "Course/01.srt"} {
		fileWriter, err := writer.CreateFormFile("file:"+name, filepath.Base(name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fileWriter.Write([]byte(name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	dataDir := t.TempDir()
	handler := testHandlerWithSettings(config.DefaultSettings(), dataDir)
	req := httptest.NewRequest(http.MethodPost, "/api/fs/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body=%s", res.Code, res.Body.String())
	}
	var payload struct {
		Files []uploadEntry `json:"files"`
		Root  string        `json:"root"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Root != filepath.Join(dataDir, "uploads", "Course") {
		t.Fatalf("root = %q, want Course upload root", payload.Root)
	}
	if len(payload.Files) != 2 {
		t.Fatalf("files len = %d, want 2", len(payload.Files))
	}
	if payload.Files[0].Name != "Course/01.mp4" {
		t.Fatalf("first name = %q, want Course/01.mp4", payload.Files[0].Name)
	}
	if payload.Files[0].URL != "/uploads/Course/01.mp4" {
		t.Fatalf("first url = %q, want /uploads/Course/01.mp4", payload.Files[0].URL)
	}
	req = httptest.NewRequest(http.MethodGet, payload.Files[0].URL, nil)
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("get nested upload status = %d, want 200", res.Code)
	}
}

func TestServeStaticAssets(t *testing.T) {
	t.Parallel()

	handler := testHandler()
	tests := []struct {
		name      string
		path      string
		content   string
		substring string
	}{
		{name: "shell", path: "/", content: "text/html", substring: "<html"},
		{name: "javascript entrypoint", path: "/app.js", content: "javascript", substring: "./js/main.js"},
		{name: "javascript module", path: "/js/main.js", content: "javascript", substring: "export function init"},
		{name: "css", path: "/style.css", content: "text/css", substring: "body"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)
			if res.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200, body=%s", res.Code, res.Body.String())
			}
			if !strings.Contains(res.Header().Get("Content-Type"), tc.content) {
				t.Fatalf("content-type = %q, want contains %q", res.Header().Get("Content-Type"), tc.content)
			}
			if !strings.Contains(res.Body.String(), tc.substring) {
				t.Fatalf("body does not contain %q", tc.substring)
			}
		})
	}
}

func TestAPISmokeContracts(t *testing.T) {
	t.Parallel()

	settings := config.DefaultSettings()
	settings.Tools = config.ToolConfig{}
	handler := testHandlerWithSettings(settings, "")

	t.Run("settings", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200, body=%s", res.Code, res.Body.String())
		}
		var got config.Settings
		if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		if got.DefaultProvider == "" {
			t.Fatal("default_provider is empty")
		}
		if len(got.Providers) == 0 {
			t.Fatal("providers list is empty")
		}
	})

	t.Run("providers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/providers", nil)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200, body=%s", res.Code, res.Body.String())
		}
		var got struct {
			Providers []config.ProviderConfig `json:"providers"`
		}
		if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		if len(got.Providers) != len(settings.Providers) {
			t.Fatalf("providers count = %d, want %d", len(got.Providers), len(settings.Providers))
		}
		for i := range got.Providers {
			if got.Providers[i].ID == "" {
				t.Fatalf("provider[%d] has empty id", i)
			}
		}
	})

	t.Run("tools", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/tools", nil)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200, body=%s", res.Code, res.Body.String())
		}
		var got struct {
			Tools []struct {
				Name      string `json:"name"`
				Command   string `json:"command"`
				Available bool   `json:"available"`
				Version   string `json:"version"`
				Error     string `json:"error"`
			} `json:"tools"`
		}
		if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		if len(got.Tools) == 0 {
			t.Fatal("tools list is empty")
		}
		for _, tool := range got.Tools {
			if tool.Name == "" {
				t.Fatal("tool name is empty")
			}
			if tool.Command != "" {
				t.Fatalf("tool %s command = %q, want empty command in test config", tool.Name, tool.Command)
			}
			if tool.Available {
				t.Fatalf("tool %s should be unavailable when command is empty in test config", tool.Name)
			}
		}
	})
}

func testHandler() http.Handler {
	return testHandlerWithDataDir("")
}

func testHandlerWithDataDir(dataDir string) http.Handler {
	settings := config.DefaultSettings()
	return testHandlerWithSettings(settings, dataDir)
}

func testHandlerWithSettings(settings config.Settings, dataDir string) http.Handler {
	return New(Dependencies{
		Logger:    slog.Default(),
		DataDir:   dataDir,
		Settings:  settings,
		Store:     &memoryStore{jobs: map[string]storage.Job{}},
		Providers: provider.NewRegistry(settings.Providers),
	})
}
