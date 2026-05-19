package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bhickta/aicli/internal/config"
)

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
		{name: "javascript entrypoint", path: "/app.js", content: "javascript", substring: "./js/core/main.js"},
		{name: "javascript module", path: "/js/core/main.js", content: "javascript", substring: "export function init"},
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
