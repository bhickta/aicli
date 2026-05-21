package model

type InboxMergeRequest struct {
	Options
}

type InboxMergeResponse struct {
	RunID          string              `json:"run_id"`
	ArchivePath    string              `json:"archive_path"`
	Processed      []InboxSourceResult `json:"processed"`
	Pending        []InboxSourceResult `json:"pending"`
	Failed         []InboxSourceResult `json:"failed"`
	SourceCount    int                 `json:"source_count"`
	SelectedCount  int                 `json:"selected_count"`
	SkippedCount   int                 `json:"skipped_count"`
	Limit          int                 `json:"limit,omitempty"`
	ProcessedCount int                 `json:"processed_count"`
	PendingCount   int                 `json:"pending_count"`
	FailedCount    int                 `json:"failed_count"`
	APICalls       APICallUsage        `json:"api_calls"`
}

type InboxSourceResult struct {
	SourcePath       string                 `json:"source_path"`
	Status           string                 `json:"status"`
	ProcessedPath    string                 `json:"processed_path,omitempty"`
	DestinationPaths []string               `json:"destination_paths,omitempty"`
	MergedCount      int                    `json:"merged_count"`
	DedupedCount     int                    `json:"deduped_count"`
	PendingCount     int                    `json:"pending_count"`
	Reason           string                 `json:"reason,omitempty"`
	Claims           []InboxClaim           `json:"claims,omitempty"`
	Ledger           []InboxClaimLedger     `json:"ledger,omitempty"`
	Diffs            []InboxDestinationDiff `json:"diffs,omitempty"`
	Validation       MergeJudge             `json:"validation,omitempty"`
}

type InboxClaim struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Source string `json:"source,omitempty"`
}

type InboxClaimLedger struct {
	ClaimID         string `json:"claim_id"`
	Status          string `json:"status"`
	DestinationPath string `json:"destination_path,omitempty"`
	Evidence        string `json:"evidence,omitempty"`
	Reason          string `json:"reason,omitempty"`
}

type InboxDestinationDiff struct {
	Path   string `json:"path"`
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
	Diff   string `json:"diff"`
}
