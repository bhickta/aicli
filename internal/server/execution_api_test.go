package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
)

type executionAPIFakeProvider struct{}

func (executionAPIFakeProvider) ID() string                   { return "fake" }
func (executionAPIFakeProvider) Health(context.Context) error { return nil }
func (executionAPIFakeProvider) ListModels(context.Context) ([]provider.Model, error) {
	return []provider.Model{{ID: "model-1", Name: "Model 1"}}, nil
}
func (executionAPIFakeProvider) Chat(context.Context, provider.ChatRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{Content: "secured"}, nil
}
func (executionAPIFakeProvider) ChatStream(context.Context, provider.ChatRequest, func(string) error) error {
	return nil
}
func (executionAPIFakeProvider) Vision(context.Context, provider.VisionRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{}, nil
}

func TestExecutionAPIRequiresServiceToken(t *testing.T) {
	handler := executionTestHandler(t)
	request := httptest.NewRequest(http.MethodPost, "/api/execution/run", bytes.NewBufferString(`{"profile":"text","prompt":"hello"}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestExecutionAPIRunsAuthenticatedProfile(t *testing.T) {
	handler := executionTestHandler(t)
	request := httptest.NewRequest(http.MethodPost, "/api/execution/run", bytes.NewBufferString(`{"profile":"text","prompt":"hello"}`))
	request.Header.Set("Authorization", "Bearer secret")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(`"content":"secured"`)) {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func executionTestHandler(t *testing.T) http.Handler {
	t.Helper()
	settings := config.Settings{ExecutionProfiles: []config.ExecutionProfile{{
		ID: "text", Capability: config.CapabilityText, Enabled: true, MaxConcurrency: 1,
		Targets: []config.ExecutionTarget{{ProviderID: "fake", Enabled: true}},
	}}}
	return New(Dependencies{
		Settings: settings, ExecutionToken: "secret",
		ProviderFor: func(string) (provider.Provider, bool) { return executionAPIFakeProvider{}, true },
	})
}
