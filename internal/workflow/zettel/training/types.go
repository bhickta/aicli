package training

import (
	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/workflow/zettel/model"
)

const (
	exportWorkflow     = "zettel-inbox-merge"
	exportStep         = "merge-final-notes"
	exportParsedFormat = "final-notes"
)

type Options = model.Options
type TrainingExportRequest = model.TrainingExportRequest
type TrainingExportResponse = model.TrainingExportResponse
type ProgressFunc = model.ProgressFunc

type Runner struct{}

type chatExample struct {
	Messages []provider.Message `json:"messages"`
}

type exportRecord struct {
	hash    string
	example chatExample
}

func New() Runner {
	return Runner{}
}
