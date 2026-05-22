package model

const (
	DefaultRootFolder           = "zettelkasten"
	DefaultDataFolder           = ".aicli-zettel-merge"
	DefaultEmbeddingModel       = "text-embedding-nomic-embed-text-v1.5"
	DefaultCandidateLimit       = 12
	DefaultEmbeddingSourceChars = 4000
	DefaultMaxMergeInputChars   = 120000
	DefaultEmbeddingBatchSize   = 128
	DefaultEmbeddingWorkers     = 4
	DefaultInboxFolder          = "inbox-to-merge"
	DefaultShorthandPromptPath  = "example_prompts.md"
)

type Options struct {
	VaultPath            string `json:"vault_path"`
	RootFolder           string `json:"root_folder"`
	DataFolder           string `json:"data_folder"`
	ProviderID           string `json:"provider_id"`
	MergeProviderID      string `json:"merge_provider_id"`
	EmbeddingProviderID  string `json:"embedding_provider_id"`
	MergeModel           string `json:"merge_model"`
	EmbeddingModel       string `json:"embedding_model"`
	CandidateLimit       int    `json:"candidate_limit"`
	EmbeddingSourceChars int    `json:"embedding_source_chars"`
	MaxMergeInputChars   int    `json:"max_merge_input_chars"`
	EmbeddingBatchSize   int    `json:"embedding_batch_size"`
	EmbeddingWorkers     int    `json:"embedding_workers"`
	InboxFolder          string `json:"inbox_folder"`
	InboxLimit           int    `json:"inbox_limit"`
	ShorthandPromptPath  string `json:"shorthand_prompt_path"`
}
