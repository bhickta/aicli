package model

type TrainingExportRequest struct {
	Options
	Strict bool `json:"strict"`
}

type TrainingExportResponse struct {
	RunID             string                `json:"run_id"`
	ArchivePath       string                `json:"archive_path"`
	TrainPath         string                `json:"train_path"`
	EvalPath          string                `json:"eval_path"`
	ShareGPTTrainPath string                `json:"sharegpt_train_path"`
	ShareGPTEvalPath  string                `json:"sharegpt_eval_path"`
	ManifestPath      string                `json:"manifest_path"`
	SourceFiles       []string              `json:"source_files"`
	Strict            bool                  `json:"strict"`
	ScannedCount      int                   `json:"scanned_count"`
	ExportedCount     int                   `json:"exported_count"`
	TrainCount        int                   `json:"train_count"`
	EvalCount         int                   `json:"eval_count"`
	DuplicateCount    int                   `json:"duplicate_count"`
	SkippedCount      int                   `json:"skipped_count"`
	SkippedByReason   map[string]int        `json:"skipped_by_reason"`
	Quality           TrainingQualityReport `json:"quality"`
	APICalls          APICallUsage          `json:"api_calls"`
}

type TrainingQualityReport struct {
	SystemPromptVariants              int `json:"system_prompt_variants"`
	PrimarySystemPromptCount          int `json:"primary_system_prompt_count"`
	ExamplesWithSemanticCandidates    int `json:"examples_with_semantic_candidates"`
	ExamplesWithoutSemanticCandidates int `json:"examples_without_semantic_candidates"`
	ExamplesWithCodeFences            int `json:"examples_with_code_fences"`
	ExamplesWithDuplicateFrontmatter  int `json:"examples_with_duplicate_frontmatter"`
	ExamplesWithBadNoteBoundaries     int `json:"examples_with_bad_note_boundaries"`
	ExamplesWithStatusOrJSONOutput    int `json:"examples_with_status_or_json_output"`
	ShortAssistantCount               int `json:"short_assistant_count"`
	LongAssistantCount                int `json:"long_assistant_count"`
	TotalFinalNotes                   int `json:"total_final_notes"`
	MaxFinalNotesPerExample           int `json:"max_final_notes_per_example"`
	MinUserChars                      int `json:"min_user_chars"`
	MaxUserChars                      int `json:"max_user_chars"`
	AverageUserChars                  int `json:"average_user_chars"`
	MinAssistantChars                 int `json:"min_assistant_chars"`
	MaxAssistantChars                 int `json:"max_assistant_chars"`
	AverageAssistantChars             int `json:"average_assistant_chars"`
}
