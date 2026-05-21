package model

type IndexRequest struct {
	Options
}

type IndexResponse struct {
	Scanned int `json:"scanned"`
	Updated int `json:"updated"`
	Reused  int `json:"reused"`
	Pruned  int `json:"pruned"`
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

type ListNotesRequest struct {
	Options
}

type ListNotesResponse struct {
	Notes []string `json:"notes"`
	Count int      `json:"count"`
}
