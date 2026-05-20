package zettel

import "strings"

func NormalizeOptions(options Options) Options {
	return normalizeOptions(options)
}

func normalizeOptions(options Options) Options {
	if strings.TrimSpace(options.RootFolder) == "" {
		options.RootFolder = DefaultRootFolder
	}
	if strings.TrimSpace(options.DataFolder) == "" {
		options.DataFolder = DefaultDataFolder
	}
	if strings.TrimSpace(options.EmbeddingModel) == "" {
		options.EmbeddingModel = DefaultEmbeddingModel
	}
	if options.CandidateLimit <= 0 {
		options.CandidateLimit = DefaultCandidateLimit
	}
	if options.ReviewThreshold <= 0 {
		options.ReviewThreshold = DefaultReviewThreshold
	}
	if options.ValidationThreshold <= 0 {
		options.ValidationThreshold = DefaultValidationThreshold
	}
	if options.EmbeddingSourceChars <= 0 {
		options.EmbeddingSourceChars = DefaultEmbeddingSourceChars
	}
	if options.CandidateJudgeChars <= 0 {
		options.CandidateJudgeChars = DefaultCandidateJudgeChars
	}
	if options.MaxMergeInputChars <= 0 {
		options.MaxMergeInputChars = DefaultMaxMergeInputChars
	}
	if options.MaxMergeRetries <= 0 {
		options.MaxMergeRetries = DefaultMaxMergeRetries
	}
	if options.EmbeddingBatchSize <= 0 {
		options.EmbeddingBatchSize = DefaultEmbeddingBatchSize
	}
	options.RootFolder = strings.Trim(strings.TrimSpace(options.RootFolder), "/")
	options.DataFolder = strings.Trim(strings.TrimSpace(options.DataFolder), "/")
	options.VaultPath = strings.TrimSpace(options.VaultPath)
	options.ProviderID = strings.TrimSpace(options.ProviderID)
	options.EmbeddingProviderID = strings.TrimSpace(options.EmbeddingProviderID)
	if options.EmbeddingProviderID == "" {
		options.EmbeddingProviderID = options.ProviderID
	}
	options.JudgeModel = strings.TrimSpace(options.JudgeModel)
	options.MergeModel = strings.TrimSpace(options.MergeModel)
	options.EmbeddingModel = strings.TrimSpace(options.EmbeddingModel)
	return options
}
