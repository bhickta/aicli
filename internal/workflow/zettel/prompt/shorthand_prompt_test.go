package prompt

import (
	"testing"

	"github.com/bhickta/aicli/internal/workflow/zettel/model"
)

func TestLoadShorthandPromptBuiltinUsesFallback(t *testing.T) {
	t.Parallel()

	got := LoadShorthandPrompt(model.Options{ShorthandPromptPath: "builtin"})
	if got != fallbackShorthandPrompt {
		t.Fatalf("loadShorthandPrompt(builtin) returned custom prompt, want fallback")
	}
}
