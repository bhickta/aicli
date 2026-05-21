package llmjson

import (
	"context"
	"testing"

	"github.com/bhickta/aicli/internal/provider"
)

type fakeJSONProvider struct {
	content string
}

func (p fakeJSONProvider) ID() string { return "fake-json" }
func (p fakeJSONProvider) Health(context.Context) error {
	return nil
}
func (p fakeJSONProvider) ListModels(context.Context) ([]provider.Model, error) {
	return nil, nil
}
func (p fakeJSONProvider) Chat(context.Context, provider.ChatRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{Content: p.content}, nil
}
func (p fakeJSONProvider) ChatStream(context.Context, provider.ChatRequest, func(string) error) error {
	return nil
}
func (p fakeJSONProvider) Vision(context.Context, provider.VisionRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{}, nil
}

func TestChatParsesLargestBalancedJSONObject(t *testing.T) {
	t.Parallel()

	type response struct {
		Claims []string `json:"claims"`
	}
	got, err := Chat[response](
		context.Background(),
		fakeJSONProvider{content: `{"debug":true}
{"claims":["kept"],"nested":{"text":"brace } inside string"}}`},
		"model",
		nil,
	)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if len(got.Claims) != 1 || got.Claims[0] != "kept" {
		t.Fatalf("Chat() = %#v, want largest valid response object", got)
	}
}

func TestExtractJSONObjectsIgnoresBracesInsideStrings(t *testing.T) {
	t.Parallel()

	objects, err := extractJSONObjects(`prefix {"text":"{not structural}","ok":true} suffix {"x":1}`)
	if err != nil {
		t.Fatalf("extractJSONObjects() error = %v", err)
	}
	if len(objects) != 2 {
		t.Fatalf("objects = %#v, want two balanced objects", objects)
	}
	if objects[0] != `{"text":"{not structural}","ok":true}` {
		t.Fatalf("first object = %q", objects[0])
	}
}
