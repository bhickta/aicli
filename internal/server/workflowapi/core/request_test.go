package core

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Ada"}`))
	rec := httptest.NewRecorder()

	got, ok := DecodeJSON[struct {
		Name string `json:"name"`
	}](rec, req)

	if !ok {
		t.Fatal("DecodeJSON returned ok=false for valid JSON")
	}
	if got.Name != "Ada" {
		t.Fatalf("DecodeJSON name = %q, want Ada", got.Name)
	}
}

func TestDecodeJSONRejectsInvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{`))
	rec := httptest.NewRecorder()

	_, ok := DecodeJSON[struct{}](rec, req)

	if ok {
		t.Fatal("DecodeJSON returned ok=true for invalid JSON")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "error") {
		t.Fatalf("body = %q, want error field", rec.Body.String())
	}
}
