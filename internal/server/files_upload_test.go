package server

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bhickta/aicli/internal/config"
)

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

	body, contentType := uploadBody(t, map[string]string{"file": "%PDF-1.7\n"})
	settings := config.DefaultSettings()
	settings.Tools = config.ToolConfig{}
	handler := testHandlerWithSettings(settings, t.TempDir())
	req := httptest.NewRequest(http.MethodPost, "/api/fs/upload", body)
	req.Header.Set("Content-Type", contentType)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body=%s", res.Code, res.Body.String())
	}
	payload := decodeUploadPayload(t, res)
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
	assertUploadServed(t, handler, payload.Files[0].URL, "%PDF-1.7\n")
}

func TestUploadFilesPreservesDroppedFolderShape(t *testing.T) {
	t.Parallel()

	body, contentType := uploadBody(t, map[string]string{
		"file:Course/01.mp4": "Course/01.mp4",
		"file:Course/01.srt": "Course/01.srt",
	})
	dataDir := t.TempDir()
	handler := testHandlerWithSettings(config.DefaultSettings(), dataDir)
	req := httptest.NewRequest(http.MethodPost, "/api/fs/upload", body)
	req.Header.Set("Content-Type", contentType)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body=%s", res.Code, res.Body.String())
	}
	payload := decodeUploadPayload(t, res)
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

func uploadBody(t *testing.T, files map[string]string) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for field, content := range files {
		name := "sample.pdf"
		if strings.HasPrefix(field, "file:") {
			name = filepath.Base(strings.TrimPrefix(field, "file:"))
		}
		fileWriter, err := writer.CreateFormFile(field, name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fileWriter.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return &body, writer.FormDataContentType()
}

func decodeUploadPayload(t *testing.T, res *httptest.ResponseRecorder) struct {
	Files []uploadEntry `json:"files"`
	Root  string        `json:"root"`
} {
	t.Helper()
	var payload struct {
		Files []uploadEntry `json:"files"`
		Root  string        `json:"root"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	return payload
}

func assertUploadServed(t *testing.T, handler http.Handler, url string, want string) {
	t.Helper()
	getReq := httptest.NewRequest(http.MethodGet, url, nil)
	getRes := httptest.NewRecorder()
	handler.ServeHTTP(getRes, getReq)
	if getRes.Code != http.StatusOK {
		t.Fatalf("get upload status = %d, want 200, body=%s", getRes.Code, getRes.Body.String())
	}
	if getRes.Body.String() != want {
		t.Fatalf("uploaded file body = %q, want %q", getRes.Body.String(), want)
	}
}
