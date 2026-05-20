package zettel

import "time"

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
	DefaultEmbeddingBatchSize   = 64
)

type Options struct {
	VaultPath            string  `json:"vault_path"`
	RootFolder           string  `json:"root_folder"`
	DataFolder           string  `json:"data_folder"`
	ProviderID           string  `json:"provider_id"`
	JudgeModel           string  `json:"judge_model"`
	MergeModel           string  `json:"merge_model"`
	EmbeddingModel       string  `json:"embedding_model"`
	CandidateLimit       int     `json:"candidate_limit"`
	ReviewThreshold      float64 `json:"review_threshold"`
	ValidationThreshold  float64 `json:"validation_threshold"`
	EmbeddingSourceChars int     `json:"embedding_source_chars"`
	CandidateJudgeChars  int     `json:"candidate_judge_chars"`
	MaxMergeInputChars   int     `json:"max_merge_input_chars"`
	MaxMergeRetries      int     `json:"max_merge_retries"`
	EmbeddingBatchSize   int     `json:"embedding_batch_size"`
}

type IndexRequest struct {
	Options
}

type IndexResponse struct {
	Scanned int `json:"scanned"`
	Updated int `json:"updated"`
	Reused  int `json:"reused"`
}

type SuggestRequest struct {
	Options
	ActivePath string `json:"active_path"`
}

type SuggestResponse struct {
	ActivePath string      `json:"active_path"`
	ActiveHash string      `json:"active_hash"`
	Candidates []Candidate `json:"candidates"`
}

type Candidate struct {
	Path              string      `json:"path"`
	Similarity        float64     `json:"similarity"`
	Confidence        float64     `json:"confidence"`
	Relationship      string      `json:"relationship"`
	Risk              string      `json:"risk"`
	Reason            string      `json:"reason"`
	SourceLineRanges  []LineRange `json:"source_line_ranges"`
	ExtractedMarkdown string      `json:"extracted_markdown"`
}

type ProposeRequest struct {
	Options
	ActivePath string      `json:"active_path"`
	Selections []Selection `json:"selections"`
}

type Selection struct {
	Path             string      `json:"path"`
	SourceLineRanges []LineRange `json:"source_line_ranges"`
}

type ProposeResponse struct {
	Proposal Proposal `json:"proposal"`
}

type ApplyRequest struct {
	Options
	Proposal Proposal `json:"proposal"`
}

type ApplyResponse struct {
	JobID       string   `json:"job_id"`
	ActivePath  string   `json:"active_path"`
	SourcePaths []string `json:"source_paths"`
	ArchivePath string   `json:"archive_path"`
}

type RollbackRequest struct {
	Options
	JobID string `json:"job_id"`
}

type RollbackResponse struct {
	JobID string `json:"job_id"`
}

type Proposal struct {
	ID                string             `json:"id"`
	CreatedAt         time.Time          `json:"created_at"`
	VaultPath         string             `json:"vault_path"`
	RootFolder        string             `json:"root_folder"`
	DataFolder        string             `json:"data_folder"`
	ActivePath        string             `json:"active_path"`
	ActiveHash        string             `json:"active_hash"`
	FinalMarkdown     string             `json:"final_markdown"`
	SourceExtractions []SourceExtraction `json:"source_extractions"`
	MergePlan         MergePlan          `json:"merge_plan"`
	Coverage          CoverageReport     `json:"coverage"`
	Judge             MergeJudge         `json:"judge"`
	Models            ProposalModels     `json:"models"`
}

type ProposalModels struct {
	Judge     string `json:"judge"`
	Merge     string `json:"merge"`
	Embedding string `json:"embedding"`
}

type SourceExtraction struct {
	Path              string      `json:"path"`
	OriginalHash      string      `json:"original_hash"`
	SourceLineRanges  []LineRange `json:"source_line_ranges"`
	ExtractedMarkdown string      `json:"extracted_markdown"`
}

type LineRange struct {
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Reason    string `json:"reason,omitempty"`
}

type MergePlan struct {
	Insertions []Insertion `json:"insertions"`
	Notes      string      `json:"notes,omitempty"`
}

type Insertion struct {
	AfterLine int    `json:"after_line"`
	Markdown  string `json:"markdown"`
	Reason    string `json:"reason,omitempty"`
}

type CoverageReport struct {
	Score                float64  `json:"score"`
	RequiredMissingCount int      `json:"required_missing_count"`
	MissingLinks         []string `json:"missing_links"`
	MissingTags          []string `json:"missing_tags"`
	MissingDates         []string `json:"missing_dates"`
	MissingNumbers       []string `json:"missing_numbers"`
	MissingHeadings      []string `json:"missing_headings"`
	MissingUniqueLines   []string `json:"missing_unique_lines"`
}

type MergeJudge struct {
	Verdict              string   `json:"verdict"`
	Score                float64  `json:"score"`
	MissingFacts         []string `json:"missing_facts"`
	UnsupportedAdditions []string `json:"unsupported_additions"`
	Notes                string   `json:"notes"`
}

type ProgressFunc = func(stage string, currentStep, totalSteps int)
