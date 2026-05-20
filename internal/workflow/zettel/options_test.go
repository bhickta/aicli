package zettel

import "testing"

func TestNormalizeOptionsBackfillsStepProvidersAndModels(t *testing.T) {
	t.Parallel()

	options := NormalizeOptions(Options{
		ProviderID: "lms",
		JudgeModel: "deepseek-reasoner",
		MergeModel: "deepseek-coder",
	})

	if options.CandidateProviderID != "lms" {
		t.Fatalf("candidate provider = %q, want lms", options.CandidateProviderID)
	}
	if options.MergeProviderID != "lms" {
		t.Fatalf("merge provider = %q, want lms", options.MergeProviderID)
	}
	if options.ValidationProviderID != "lms" {
		t.Fatalf("validation provider = %q, want lms", options.ValidationProviderID)
	}
	if options.EmbeddingProviderID != "lms" {
		t.Fatalf("embedding provider = %q, want lms", options.EmbeddingProviderID)
	}
	if options.CandidateModel != "deepseek-reasoner" {
		t.Fatalf("candidate model = %q, want judge model fallback", options.CandidateModel)
	}
	if options.ValidationModel != "deepseek-reasoner" {
		t.Fatalf("validation model = %q, want judge model fallback", options.ValidationModel)
	}
}

func TestNormalizeOptionsKeepsExplicitStepChoices(t *testing.T) {
	t.Parallel()

	options := NormalizeOptions(Options{
		ProviderID:           "legacy",
		CandidateProviderID:  "candidate",
		MergeProviderID:      "merge",
		ValidationProviderID: "validation",
		EmbeddingProviderID:  "embedding",
		JudgeModel:           "legacy-judge",
		CandidateModel:       "candidate-model",
		MergeModel:           "merge-model",
		ValidationModel:      "validation-model",
		EmbeddingModel:       "embedding-model",
	})

	if options.CandidateProviderID != "candidate" || options.MergeProviderID != "merge" || options.ValidationProviderID != "validation" {
		t.Fatalf("providers = %#v, want explicit step provider IDs", options)
	}
	if options.CandidateModel != "candidate-model" || options.MergeModel != "merge-model" || options.ValidationModel != "validation-model" {
		t.Fatalf("models = %#v, want explicit step models", options)
	}
}
