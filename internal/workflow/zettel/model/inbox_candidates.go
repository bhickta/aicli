package model

type InboxCandidatePreviewRequest struct {
	Options
}

type InboxCandidatePreviewResponse struct {
	Sources       []InboxCandidateSource `json:"sources"`
	SourceCount   int                    `json:"source_count"`
	SelectedCount int                    `json:"selected_count"`
	SkippedCount  int                    `json:"skipped_count"`
	Limit         int                    `json:"limit,omitempty"`
	APICalls      APICallUsage           `json:"api_calls"`
}

type InboxCandidateSource struct {
	SourcePath    string           `json:"source_path"`
	SourceExcerpt string           `json:"source_excerpt,omitempty"`
	Candidates    []InboxCandidate `json:"candidates"`
	Error         string           `json:"error,omitempty"`
}

type InboxCandidate struct {
	Path       string  `json:"path"`
	Similarity float64 `json:"similarity"`
	Excerpt    string  `json:"excerpt,omitempty"`
}
