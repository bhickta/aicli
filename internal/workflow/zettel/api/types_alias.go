package zettel

import "github.com/bhickta/aicli/internal/workflow/zettel/model"

const (
	DefaultRootFolder           = model.DefaultRootFolder
	DefaultDataFolder           = model.DefaultDataFolder
	DefaultEmbeddingModel       = model.DefaultEmbeddingModel
	DefaultCandidateLimit       = model.DefaultCandidateLimit
	DefaultEmbeddingSourceChars = model.DefaultEmbeddingSourceChars
	DefaultMaxMergeInputChars   = model.DefaultMaxMergeInputChars
	DefaultEmbeddingBatchSize   = model.DefaultEmbeddingBatchSize
	DefaultEmbeddingWorkers     = model.DefaultEmbeddingWorkers
	DefaultInboxFolder          = model.DefaultInboxFolder
	DefaultShorthandPromptPath  = model.DefaultShorthandPromptPath
)

type Options = model.Options
type IndexRequest = model.IndexRequest
type IndexResponse = model.IndexResponse
type ListNotesRequest = model.ListNotesRequest
type ListNotesResponse = model.ListNotesResponse
type InboxCandidatePreviewRequest = model.InboxCandidatePreviewRequest
type InboxCandidatePreviewResponse = model.InboxCandidatePreviewResponse
type InboxCandidateSource = model.InboxCandidateSource
type InboxCandidate = model.InboxCandidate
type InboxMergeRequest = model.InboxMergeRequest
type InboxMergeResponse = model.InboxMergeResponse
type InboxSourceResult = model.InboxSourceResult
type InboxClaim = model.InboxClaim
type InboxClaimLedger = model.InboxClaimLedger
type InboxDestinationDiff = model.InboxDestinationDiff
type MetadataRequest = model.MetadataRequest
type MetadataResponse = model.MetadataResponse
type MetadataNoteResult = model.MetadataNoteResult
type TrainingExportRequest = model.TrainingExportRequest
type TrainingExportResponse = model.TrainingExportResponse
type RollbackRequest = model.RollbackRequest
type RollbackResponse = model.RollbackResponse
type ProgressFunc = model.ProgressFunc
type APICallUsage = model.APICallUsage
type ProviderAPICallUsage = model.ProviderAPICallUsage
