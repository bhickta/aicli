package zettel

import "testing"

func TestNormalizeOptionsBackfillsMergeAndEmbeddingProviders(t *testing.T) {
	t.Parallel()

	options := NormalizeOptions(Options{
		ProviderID: "lms",
		MergeModel: "deepseek-coder",
	})

	if options.MergeProviderID != "lms" {
		t.Fatalf("merge provider = %q, want lms", options.MergeProviderID)
	}
	if options.EmbeddingProviderID != "lms" {
		t.Fatalf("embedding provider = %q, want lms", options.EmbeddingProviderID)
	}
}

func TestNormalizeOptionsKeepsExplicitMergeAndEmbeddingChoices(t *testing.T) {
	t.Parallel()

	options := NormalizeOptions(Options{
		ProviderID:          "legacy",
		MergeProviderID:     "merge",
		EmbeddingProviderID: "embedding",
		MergeModel:          "merge-model",
		EmbeddingModel:      "embedding-model",
	})

	if options.MergeProviderID != "merge" || options.EmbeddingProviderID != "embedding" {
		t.Fatalf("providers = %#v, want explicit provider IDs", options)
	}
	if options.MergeModel != "merge-model" || options.EmbeddingModel != "embedding-model" {
		t.Fatalf("models = %#v, want explicit models", options)
	}
}

func TestNormalizeOptionsClampsNegativeInboxLimit(t *testing.T) {
	t.Parallel()

	options := NormalizeOptions(Options{InboxLimit: -5})
	if options.InboxLimit != 0 {
		t.Fatalf("inbox limit = %d, want full inbox sentinel", options.InboxLimit)
	}
}
