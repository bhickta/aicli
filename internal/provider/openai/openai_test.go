package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
)

func TestOpenAICompatibleListModels(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path = %s, want /v1/models", r.URL.Path)
		}
		w.Write([]byte(`{"data":[{"id":"model-a"}]}`))
	}))
	defer srv.Close()

	p := NewCompatible(config.ProviderConfig{
		ID:      "test",
		BaseURL: srv.URL + "/v1",
		APIKey:  "key",
	}, srv.Client())

	models, err := p.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	if len(models) != 1 || models[0].ID != "model-a" {
		t.Fatalf("models = %#v, want model-a", models)
	}
}

func TestOpenAICompatibleListModelsAddsV1WhenMissing(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path = %s, want /v1/models", r.URL.Path)
		}
		w.Write([]byte(`{"data":[{"id":"model-a"}]}`))
	}))
	defer srv.Close()

	p := NewCompatible(config.ProviderConfig{
		ID:      "test",
		BaseURL: srv.URL,
	}, srv.Client())

	models, err := p.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	if len(models) != 1 || models[0].ID != "model-a" {
		t.Fatalf("models = %#v, want model-a", models)
	}
}

func TestOpenAICompatibleListModelsPreservesGeminiOpenAIBase(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1beta/openai/models" {
			t.Fatalf("path = %s, want /v1beta/openai/models", r.URL.Path)
		}
		w.Write([]byte(`{"data":[{"id":"models/gemini-2.5-flash"}]}`))
	}))
	defer srv.Close()

	p := NewCompatible(config.ProviderConfig{
		ID:      "gemini",
		BaseURL: srv.URL + "/v1beta/openai",
	}, srv.Client())

	models, err := p.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	if len(models) != 1 || models[0].ID != "gemini-2.5-flash" {
		t.Fatalf("models = %#v, want gemini-2.5-flash", models)
	}
}

func TestOpenAICompatibleListModelsAppliesModelFilter(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path = %s, want /v1/models", r.URL.Path)
		}
		w.Write([]byte(`{"data":[{"id":"gpt-5.2-codex"},{"id":"text-embedding-3-large"}]}`))
	}))
	defer srv.Close()

	p := NewCompatible(config.ProviderConfig{
		ID:          "codex",
		BaseURL:     srv.URL + "/v1",
		ModelFilter: "codex",
	}, srv.Client())

	models, err := p.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	if len(models) != 1 || models[0].ID != "gpt-5.2-codex" {
		t.Fatalf("models = %#v, want only gpt-5.2-codex", models)
	}
}

func TestOpenAICompatibleVLLMIsLocalModelServer(t *testing.T) {
	t.Parallel()

	p := NewCompatible(config.ProviderConfig{
		ID:      "custom-vllm",
		Type:    "vllm",
		BaseURL: "http://example.test/v1",
	}, http.DefaultClient)

	local, ok := any(p).(provider.LocalModelServer)
	if !ok {
		t.Fatal("OpenAICompatible does not implement provider.LocalModelServer")
	}
	if !local.LocalModelServer() {
		t.Fatal("LocalModelServer() = false, want true for vllm")
	}
}

func TestOpenAICompatibleGeminiDocumentParsesUsage(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/models/gemini-flash-lite-latest:generateContent") {
			t.Fatalf("path = %s, want Gemini generateContent path", r.URL.Path)
		}
		w.Write([]byte(`{
			"candidates":[{"content":{"parts":[{"text":"done"}]},"finishReason":"STOP"}],
			"usageMetadata":{
				"promptTokenCount":100,
				"cachedContentTokenCount":40,
				"candidatesTokenCount":25,
				"thoughtsTokenCount":3,
				"totalTokenCount":128
			}
		}`))
	}))
	defer srv.Close()

	p := NewCompatible(config.ProviderConfig{
		ID:      "gemini",
		BaseURL: srv.URL + "/v1beta/openai",
		APIKey:  "key",
	}, srv.Client())

	res, err := p.Document(context.Background(), provider.DocumentRequest{
		Model:    "models/gemini-flash-lite-latest",
		Prompt:   "read",
		Data:     []byte("pdf"),
		MIMEType: "application/pdf",
	})
	if err != nil {
		t.Fatalf("Document() error = %v", err)
	}
	if res.Content != "done" || res.Usage == nil {
		t.Fatalf("response = %#v, want content and usage", res)
	}
	if res.Usage.InputTokens != 100 || res.Usage.CachedInputTokens != 40 || res.Usage.OutputTokens != 25 || res.Usage.ReasoningOutputTokens != 3 || res.Usage.TotalTokens != 128 {
		t.Fatalf("usage = %#v, want Gemini usage mapped", res.Usage)
	}
}

func TestOpenAICompatibleUsesAPIKeyEnv(t *testing.T) {
	t.Setenv("AICLI_TEST_API_KEY", "env-key")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer env-key" {
			t.Fatalf("Authorization = %q, want Bearer env-key", got)
		}
		w.Write([]byte(`{"data":[{"id":"model-a"}]}`))
	}))
	defer srv.Close()

	p := NewCompatible(config.ProviderConfig{
		ID:        "test",
		BaseURL:   srv.URL + "/v1",
		APIKeyEnv: "AICLI_TEST_API_KEY",
	}, srv.Client())

	if _, err := p.ListModels(context.Background()); err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
}

func TestOpenAIResponsesChat(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("path = %s, want /v1/responses", r.URL.Path)
		}
		var body struct {
			Model           string             `json:"model"`
			Input           []provider.Message `json:"input"`
			Store           bool               `json:"store"`
			MaxOutputTokens int                `json:"max_output_tokens"`
			Reasoning       struct {
				Effort string `json:"effort"`
			} `json:"reasoning"`
			Text struct {
				Verbosity string `json:"verbosity"`
			} `json:"text"`
			PromptCacheKey       string `json:"prompt_cache_key"`
			PromptCacheRetention string `json:"prompt_cache_retention"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Model != "gpt-5.2-codex" || len(body.Input) != 1 || body.Input[0].Content != "fix this" {
			t.Fatalf("body = %#v", body)
		}
		if body.Store {
			t.Fatal("store = true, want false")
		}
		if body.MaxOutputTokens != 256 || body.Reasoning.Effort != "high" || body.Text.Verbosity != "low" {
			t.Fatalf("responses controls = %#v", body)
		}
		if body.PromptCacheKey != "aicli-codex-gpt-5-2-codex" {
			t.Fatalf("prompt_cache_key = %q, want default model-aware key", body.PromptCacheKey)
		}
		if body.PromptCacheRetention != "" {
			t.Fatalf("prompt_cache_retention = %q, want empty without config", body.PromptCacheRetention)
		}
		w.Write([]byte(`{"output":[{"type":"message","content":[{"type":"output_text","text":"done"}]}]}`))
	}))
	defer srv.Close()

	p := NewCompatible(config.ProviderConfig{
		ID:      "codex",
		Type:    "openai-responses",
		BaseURL: srv.URL + "/v1",
	}, srv.Client())

	res, err := p.Chat(context.Background(), provider.ChatRequest{
		Model:           "gpt-5.2-codex",
		Messages:        []provider.Message{{Role: "user", Content: "fix this"}},
		MaxTokens:       256,
		ReasoningEffort: "high",
		TextVerbosity:   "low",
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if res.Content != "done" {
		t.Fatalf("content = %q, want done", res.Content)
	}
}

func TestOpenAIResponsesChatAppliesPromptCacheConfigAndUsage(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			PromptCacheKey       string             `json:"prompt_cache_key"`
			PromptCacheRetention string             `json:"prompt_cache_retention"`
			Input                []provider.Message `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.PromptCacheKey != "aicli-codex" || body.PromptCacheRetention != "24h" {
			t.Fatalf("prompt cache = %q/%q, want configured cache", body.PromptCacheKey, body.PromptCacheRetention)
		}
		if len(body.Input) != 1 || body.Input[0].Content != " hello\n" || body.Input[0].Role != "user" {
			t.Fatalf("normalized input = %#v, want one nonblank user message with content preserved", body.Input)
		}
		w.Write([]byte(`{
			"output_text":"cached",
			"usage":{
				"input_tokens":100,
				"input_tokens_details":{"cached_tokens":80},
				"output_tokens":20,
				"output_tokens_details":{"reasoning_tokens":7},
				"total_tokens":120
			}
		}`))
	}))
	defer srv.Close()

	p := NewCompatible(config.ProviderConfig{
		ID:                   "codex",
		Type:                 "openai-responses",
		BaseURL:              srv.URL + "/v1",
		Model:                "gpt-5.2-codex",
		PromptCacheKey:       "aicli-codex",
		PromptCacheRetention: "24h",
	}, srv.Client())

	res, err := p.Chat(context.Background(), provider.ChatRequest{
		Messages: []provider.Message{
			{Role: "user", Content: "   "},
			{Role: "", Content: " hello\n"},
		},
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if res.Content != "cached" {
		t.Fatalf("content = %q, want cached", res.Content)
	}
	if res.Usage == nil || res.Usage.CachedInputTokens != 80 || res.Usage.ReasoningOutputTokens != 7 {
		t.Fatalf("usage = %#v, want cached/reasoning token usage", res.Usage)
	}
}

func TestOpenAIResponsesChatMissingAuthGivesCodexProHint(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"message":"Missing bearer or basic authentication in header"}}`))
	}))
	defer srv.Close()

	p := NewCompatible(config.ProviderConfig{
		ID:        "codex",
		Type:      "openai-responses",
		BaseURL:   srv.URL + "/v1",
		APIKeyEnv: "OPENAI_API_KEY",
		Model:     "gpt-5.2-codex",
	}, srv.Client())

	_, err := p.Chat(context.Background(), provider.ChatRequest{
		Messages: []provider.Message{{Role: "user", Content: "fix this"}},
	})
	if err == nil {
		t.Fatal("Chat() error = nil, want authentication hint")
	}
	got := err.Error()
	for _, want := range []string{"missing api authentication", "OPENAI_API_KEY", "Codex CLI / Pro"} {
		if !strings.Contains(got, want) {
			t.Fatalf("error = %q, want contains %q", got, want)
		}
	}
	if strings.Contains(got, "Missing bearer") {
		t.Fatalf("error = %q, should not expose raw missing bearer response as primary guidance", got)
	}
}

func TestOpenAICompatibleChatPreservesFinishReason(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %s, want /v1/chat/completions", r.URL.Path)
		}
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"partial"},"finish_reason":"length"}]}`))
	}))
	defer srv.Close()

	p := NewCompatible(config.ProviderConfig{ID: "test", BaseURL: srv.URL + "/v1", Model: "model"}, srv.Client())
	res, err := p.Chat(context.Background(), provider.ChatRequest{Messages: []provider.Message{{Role: "user", Content: "hi"}}})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if res.Content != "partial" || res.FinishReason != "length" {
		t.Fatalf("response = %#v, want content and finish reason", res)
	}
}

func TestOpenAICompatibleVisionSendsMultipleImages(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %s, want /v1/chat/completions", r.URL.Path)
		}
		var body struct {
			Messages []struct {
				Content []struct {
					Type string `json:"type"`
				} `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		imageParts := 0
		textParts := 0
		for _, part := range body.Messages[0].Content {
			if part.Type == "image_url" {
				imageParts++
			}
			if part.Type == "text" {
				textParts++
			}
		}
		if imageParts != 2 || textParts < 3 {
			t.Fatalf("content parts = %#v, want prompt + labels + 2 images", body.Messages[0].Content)
		}
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"done"}}]}`))
	}))
	defer srv.Close()

	p := NewCompatible(config.ProviderConfig{ID: "gemini", BaseURL: srv.URL + "/v1", Model: "model"}, srv.Client())
	res, err := p.Vision(context.Background(), provider.VisionRequest{
		Prompt: "ocr",
		Images: []provider.VisionImage{
			{Name: "page-1", Image: []byte("one"), MIMEType: "image/jpeg"},
			{Name: "page-2", Image: []byte("two"), MIMEType: "image/jpeg"},
		},
	})
	if err != nil {
		t.Fatalf("Vision() error = %v", err)
	}
	if res.Content != "done" {
		t.Fatalf("content = %q, want done", res.Content)
	}
}

func TestOpenAICompatibleDocumentUsesGeminiGenerateContent(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1beta/models/gemini-2.5-flash:generateContent" {
			t.Fatalf("path = %s, want Gemini generateContent path", r.URL.Path)
		}
		if got := r.Header.Get("x-goog-api-key"); got != "key" {
			t.Fatalf("x-goog-api-key = %q, want key", got)
		}
		var body struct {
			Contents []struct {
				Parts []map[string]any `json:"parts"`
			} `json:"contents"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body.Contents) != 1 || len(body.Contents[0].Parts) != 2 {
			t.Fatalf("body = %#v, want prompt and PDF parts", body)
		}
		w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"done"}]},"finishReason":"STOP"}]}`))
	}))
	defer srv.Close()

	p := NewCompatible(config.ProviderConfig{
		ID:      "gemini",
		BaseURL: srv.URL + "/v1beta/openai",
		APIKey:  "key",
	}, srv.Client())
	res, err := p.Document(context.Background(), provider.DocumentRequest{
		Model:    "gemini-2.5-flash",
		Prompt:   "analyze",
		Data:     []byte("%PDF"),
		MIMEType: "application/pdf",
	})
	if err != nil {
		t.Fatalf("Document() error = %v", err)
	}
	if res.Content != "done" || res.FinishReason != "STOP" {
		t.Fatalf("response = %#v, want content and finish reason", res)
	}
}

func TestOpenAICompatibleUnloadLMStudioUsesInstanceIDOnly(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/models/unload" {
			t.Fatalf("path = %s, want /api/v1/models/unload", r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["instance_id"] != "unlimited-ocr" {
			t.Fatalf("body = %#v, want instance_id", body)
		}
		if _, ok := body["model"]; ok {
			t.Fatalf("body = %#v, should not include rejected model key", body)
		}
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	p := NewCompatible(config.ProviderConfig{ID: "lms", BaseURL: srv.URL + "/v1"}, srv.Client())
	if err := p.UnloadModel(context.Background(), "unlimited-ocr"); err != nil {
		t.Fatalf("UnloadModel() error = %v", err)
	}
}

func TestOpenAICompatibleEmbeddings(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			t.Fatalf("path = %s, want /v1/embeddings", r.URL.Path)
		}
		var body struct {
			Model string   `json:"model"`
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Model != "embed" || len(body.Input) != 2 {
			t.Fatalf("body = %#v", body)
		}
		w.Write([]byte(`{"data":[{"index":1,"embedding":[0,1]},{"index":0,"embedding":[1,0]}]}`))
	}))
	defer srv.Close()

	p := NewCompatible(config.ProviderConfig{
		ID:      "test",
		BaseURL: srv.URL + "/v1",
	}, srv.Client())

	res, err := p.Embeddings(context.Background(), provider.EmbeddingRequest{
		Model:  "embed",
		Inputs: []string{"a", "b"},
	})
	if err != nil {
		t.Fatalf("Embeddings() error = %v", err)
	}
	if len(res.Vectors) != 2 || res.Vectors[0][0] != 1 || res.Vectors[1][1] != 1 {
		t.Fatalf("vectors = %#v", res.Vectors)
	}
}
