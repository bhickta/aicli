package model

import (
	"path/filepath"
	"strings"
)

func NormalizeOptions(options Options) Options {
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
	if strings.TrimSpace(options.InboxFolder) == "" {
		options.InboxFolder = DefaultInboxFolder
	}
	if options.InboxLimit < 0 {
		options.InboxLimit = 0
	}
	if strings.TrimSpace(options.ShorthandPromptPath) == "" {
		options.ShorthandPromptPath = DefaultShorthandPromptPath
	}
	options.RootFolder = strings.Trim(strings.TrimSpace(options.RootFolder), "/")
	options.DataFolder = strings.TrimSpace(options.DataFolder)
	if !filepath.IsAbs(options.DataFolder) {
		options.DataFolder = strings.Trim(options.DataFolder, "/")
	}
	options.InboxFolder = strings.Trim(strings.TrimSpace(options.InboxFolder), "/")
	options.ShorthandPromptPath = strings.TrimSpace(options.ShorthandPromptPath)
	options.VaultPath = strings.TrimSpace(options.VaultPath)
	options.ProviderID = strings.TrimSpace(options.ProviderID)
	options.CandidateProviderID = strings.TrimSpace(options.CandidateProviderID)
	if options.CandidateProviderID == "" {
		options.CandidateProviderID = options.ProviderID
	}
	if options.ProviderID == "" {
		options.ProviderID = options.CandidateProviderID
	}
	options.MergeProviderID = strings.TrimSpace(options.MergeProviderID)
	if options.MergeProviderID == "" {
		options.MergeProviderID = options.ProviderID
	}
	options.ValidationProviderID = strings.TrimSpace(options.ValidationProviderID)
	if options.ValidationProviderID == "" {
		options.ValidationProviderID = options.CandidateProviderID
	}
	options.EmbeddingProviderID = strings.TrimSpace(options.EmbeddingProviderID)
	if options.EmbeddingProviderID == "" {
		options.EmbeddingProviderID = options.ProviderID
	}
	options.JudgeModel = strings.TrimSpace(options.JudgeModel)
	options.CandidateModel = strings.TrimSpace(options.CandidateModel)
	if options.CandidateModel == "" {
		options.CandidateModel = options.JudgeModel
	}
	if options.JudgeModel == "" {
		options.JudgeModel = options.CandidateModel
	}
	options.MergeModel = strings.TrimSpace(options.MergeModel)
	options.ValidationModel = strings.TrimSpace(options.ValidationModel)
	if options.ValidationModel == "" {
		options.ValidationModel = options.JudgeModel
	}
	options.EmbeddingModel = strings.TrimSpace(options.EmbeddingModel)
	return options
}
