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
	if options.EmbeddingSourceChars <= 0 {
		options.EmbeddingSourceChars = DefaultEmbeddingSourceChars
	}
	if options.MaxMergeInputChars <= 0 {
		options.MaxMergeInputChars = DefaultMaxMergeInputChars
	}
	if options.EmbeddingBatchSize <= 0 {
		options.EmbeddingBatchSize = DefaultEmbeddingBatchSize
	}
	if options.EmbeddingWorkers <= 0 {
		options.EmbeddingWorkers = DefaultEmbeddingWorkers
	}
	if strings.TrimSpace(options.InboxFolder) == "" {
		options.InboxFolder = DefaultInboxFolder
	}
	if options.InboxLimit < 0 {
		options.InboxLimit = 0
	}
	if options.InboxWorkers <= 0 {
		options.InboxWorkers = DefaultInboxWorkers
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
	options.MergeProviderID = strings.TrimSpace(options.MergeProviderID)
	if options.MergeProviderID == "" {
		options.MergeProviderID = options.ProviderID
	}
	if options.ProviderID == "" {
		options.ProviderID = options.MergeProviderID
	}
	options.EmbeddingProviderID = strings.TrimSpace(options.EmbeddingProviderID)
	if options.EmbeddingProviderID == "" {
		options.EmbeddingProviderID = options.ProviderID
	}
	options.MergeModel = strings.TrimSpace(options.MergeModel)
	options.EmbeddingModel = strings.TrimSpace(options.EmbeddingModel)
	return options
}
