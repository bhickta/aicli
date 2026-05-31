package model

type MetadataRequest struct {
	Options
	MetadataFolder    string `json:"metadata_folder"`
	MetadataLimit     int    `json:"metadata_limit"`
	MetadataWorkers   int    `json:"metadata_workers"`
	MetadataOverwrite bool   `json:"metadata_overwrite"`
}

type MetadataResponse struct {
	RunID          string               `json:"run_id"`
	ArchivePath    string               `json:"archive_path"`
	Processed      []MetadataNoteResult `json:"processed"`
	Skipped        []MetadataNoteResult `json:"skipped"`
	Failed         []MetadataNoteResult `json:"failed"`
	SourceCount    int                  `json:"source_count"`
	SelectedCount  int                  `json:"selected_count"`
	SkippedCount   int                  `json:"skipped_count"`
	Limit          int                  `json:"limit,omitempty"`
	ProcessedCount int                  `json:"processed_count"`
	FailedCount    int                  `json:"failed_count"`
	APICalls       APICallUsage         `json:"api_calls"`
}

type MetadataNoteResult struct {
	Path            string                `json:"path"`
	Status          string                `json:"status"`
	Reason          string                `json:"reason,omitempty"`
	Title           string                `json:"title,omitempty"`
	SummaryKeywords string                `json:"summary_keywords,omitempty"`
	RecallQuestions []string              `json:"recall_questions,omitempty"`
	Diff            *InboxDestinationDiff `json:"diff,omitempty"`
}
