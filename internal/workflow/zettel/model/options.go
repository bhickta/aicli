package model

const (
	DefaultRootFolder           = "zettelkasten"
	DefaultDataFolder           = ".aicli-zettel-merge"
	DefaultEmbeddingModel       = "text-embedding-nomic-embed-text-v1.5"
	DefaultCandidateLimit       = 12
	DefaultReviewThreshold      = 0.85
	DefaultValidationThreshold  = 0.98
	DefaultEmbeddingSourceChars = 4000
	DefaultCandidateJudgeChars  = 2500
	DefaultMaxMergeInputChars   = 120000
	DefaultMaxMergeRetries      = 2
	DefaultEmbeddingBatchSize   = 128
	DefaultEmbeddingWorkers     = 4
	DefaultInboxFolder          = "inbox-to-merge"
	DefaultShorthandPromptPath  = "example_prompts.md"
)

type Options struct {
	VaultPath            string  `json:"vault_path"`
	RootFolder           string  `json:"root_folder"`
	DataFolder           string  `json:"data_folder"`
	ProviderID           string  `json:"provider_id"`
	CandidateProviderID  string  `json:"candidate_provider_id"`
	MergeProviderID      string  `json:"merge_provider_id"`
	ValidationProviderID string  `json:"validation_provider_id"`
	EmbeddingProviderID  string  `json:"embedding_provider_id"`
	JudgeModel           string  `json:"judge_model"`
	CandidateModel       string  `json:"candidate_model"`
	MergeModel           string  `json:"merge_model"`
	ValidationModel      string  `json:"validation_model"`
	EmbeddingModel       string  `json:"embedding_model"`
	CandidateLimit       int     `json:"candidate_limit"`
	ReviewThreshold      float64 `json:"review_threshold"`
	ValidationThreshold  float64 `json:"validation_threshold"`
	EmbeddingSourceChars int     `json:"embedding_source_chars"`
	CandidateJudgeChars  int     `json:"candidate_judge_chars"`
	MaxMergeInputChars   int     `json:"max_merge_input_chars"`
	MaxMergeRetries      int     `json:"max_merge_retries"`
	EmbeddingBatchSize   int     `json:"embedding_batch_size"`
	EmbeddingWorkers     int     `json:"embedding_workers"`
	InboxFolder          string  `json:"inbox_folder"`
	InboxLimit           int     `json:"inbox_limit"`
	AdoptUnmatchedInbox  bool    `json:"adopt_unmatched_inbox"`
	ShorthandPromptPath  string  `json:"shorthand_prompt_path"`
}
