package model

type TrainingExportRequest struct {
	Options
}

type TrainingExportResponse struct {
	RunID           string         `json:"run_id"`
	ArchivePath     string         `json:"archive_path"`
	TrainPath       string         `json:"train_path"`
	EvalPath        string         `json:"eval_path"`
	ManifestPath    string         `json:"manifest_path"`
	SourceFiles     []string       `json:"source_files"`
	ScannedCount    int            `json:"scanned_count"`
	ExportedCount   int            `json:"exported_count"`
	TrainCount      int            `json:"train_count"`
	EvalCount       int            `json:"eval_count"`
	DuplicateCount  int            `json:"duplicate_count"`
	SkippedCount    int            `json:"skipped_count"`
	SkippedByReason map[string]int `json:"skipped_by_reason"`
	APICalls        APICallUsage   `json:"api_calls"`
}
