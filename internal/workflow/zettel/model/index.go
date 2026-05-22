package model

type IndexRequest struct {
	Options
}

type IndexResponse struct {
	Scanned  int          `json:"scanned"`
	Updated  int          `json:"updated"`
	Reused   int          `json:"reused"`
	Pruned   int          `json:"pruned"`
	APICalls APICallUsage `json:"api_calls"`
}

type ListNotesRequest struct {
	Options
}

type ListNotesResponse struct {
	Notes []string `json:"notes"`
	Count int      `json:"count"`
}
