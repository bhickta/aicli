package model

type RollbackRequest struct {
	Options
	JobID string `json:"job_id"`
}

type RollbackResponse struct {
	JobID    string       `json:"job_id"`
	APICalls APICallUsage `json:"api_calls"`
}
