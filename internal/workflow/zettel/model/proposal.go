package model

import "time"

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
	Proposal Proposal     `json:"proposal"`
	APICalls APICallUsage `json:"api_calls"`
}

type ApplyRequest struct {
	Options
	Proposal Proposal `json:"proposal"`
}

type ApplyResponse struct {
	JobID       string       `json:"job_id"`
	ActivePath  string       `json:"active_path"`
	SourcePaths []string     `json:"source_paths"`
	ArchivePath string       `json:"archive_path"`
	APICalls    APICallUsage `json:"api_calls"`
}

type RollbackRequest struct {
	Options
	JobID string `json:"job_id"`
}

type RollbackResponse struct {
	JobID    string       `json:"job_id"`
	APICalls APICallUsage `json:"api_calls"`
}

type Proposal struct {
	ID                string             `json:"id"`
	CreatedAt         time.Time          `json:"created_at"`
	VaultPath         string             `json:"vault_path"`
	RootFolder        string             `json:"root_folder"`
	DataFolder        string             `json:"data_folder"`
	ActivePath        string             `json:"active_path"`
	ActiveHash        string             `json:"active_hash"`
	ActiveMarkdown    string             `json:"active_markdown,omitempty"`
	FinalMarkdown     string             `json:"final_markdown"`
	SourceExtractions []SourceExtraction `json:"source_extractions"`
	MergePlan         MergePlan          `json:"merge_plan"`
	Coverage          CoverageReport     `json:"coverage"`
	Judge             MergeJudge         `json:"judge"`
	Models            ProposalModels     `json:"models"`
	Providers         ProposalProviders  `json:"providers"`
	APICalls          APICallUsage       `json:"api_calls"`
}

type ProposalModels struct {
	Judge           string `json:"judge"`
	CandidateJudge  string `json:"candidate_judge"`
	Merge           string `json:"merge"`
	ValidationJudge string `json:"validation_judge"`
	Embedding       string `json:"embedding"`
}

type ProposalProviders struct {
	CandidateJudge  string `json:"candidate_judge"`
	Merge           string `json:"merge"`
	ValidationJudge string `json:"validation_judge"`
	Embedding       string `json:"embedding"`
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
