package metadata

import (
	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/workflow/zettel/model"
	"github.com/bhickta/aicli/internal/workflow/zettel/vaultfs"
)

const (
	StatusProcessed = "processed"
	StatusSkipped   = "skipped"
	StatusFailed    = "failed"
)

type Options = model.Options
type MetadataRequest = model.MetadataRequest
type MetadataResponse = model.MetadataResponse
type MetadataNoteResult = model.MetadataNoteResult
type ProgressFunc = model.ProgressFunc

type vault = vaultfs.Vault

type Runner struct {
	provider provider.Provider
}

func New(p provider.Provider) Runner {
	return Runner{provider: p}
}

type generatedMetadata struct {
	Title           string   `json:"title"`
	SummaryKeywords string   `json:"summary_keywords"`
	RecallQuestions []string `json:"recall_questions"`
}
