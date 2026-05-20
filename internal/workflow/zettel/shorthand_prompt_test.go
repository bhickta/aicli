package zettel

import "testing"

func TestLoadShorthandPromptBuiltinUsesFallback(t *testing.T) {
	t.Parallel()

	got := loadShorthandPrompt(Options{ShorthandPromptPath: "builtin"})
	if got != fallbackShorthandPrompt {
		t.Fatalf("loadShorthandPrompt(builtin) returned custom prompt, want fallback")
	}
}
