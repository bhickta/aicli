package zettel

import "github.com/bhickta/aicli/internal/workflow/zettel/model"

const (
	DefaultRootFolder           = model.DefaultRootFolder
	DefaultDataFolder           = model.DefaultDataFolder
	DefaultEmbeddingModel       = model.DefaultEmbeddingModel
	DefaultCandidateLimit       = model.DefaultCandidateLimit
	DefaultReviewThreshold      = model.DefaultReviewThreshold
	DefaultValidationThreshold  = model.DefaultValidationThreshold
	DefaultEmbeddingSourceChars = model.DefaultEmbeddingSourceChars
	DefaultCandidateJudgeChars  = model.DefaultCandidateJudgeChars
	DefaultMaxMergeInputChars   = model.DefaultMaxMergeInputChars
	DefaultMaxMergeRetries      = model.DefaultMaxMergeRetries
	DefaultEmbeddingBatchSize   = model.DefaultEmbeddingBatchSize
	DefaultInboxFolder          = model.DefaultInboxFolder
	DefaultShorthandPromptPath  = model.DefaultShorthandPromptPath
)

type Options = model.Options
type IndexRequest = model.IndexRequest
type IndexResponse = model.IndexResponse
type SuggestRequest = model.SuggestRequest
type SuggestResponse = model.SuggestResponse
type Candidate = model.Candidate
type ListNotesRequest = model.ListNotesRequest
type ListNotesResponse = model.ListNotesResponse
type InboxMergeRequest = model.InboxMergeRequest
type InboxMergeResponse = model.InboxMergeResponse
type InboxSourceResult = model.InboxSourceResult
type InboxClaim = model.InboxClaim
type InboxClaimLedger = model.InboxClaimLedger
type InboxDestinationDiff = model.InboxDestinationDiff
type ProposeRequest = model.ProposeRequest
type Selection = model.Selection
type ProposeResponse = model.ProposeResponse
type ApplyRequest = model.ApplyRequest
type ApplyResponse = model.ApplyResponse
type RollbackRequest = model.RollbackRequest
type RollbackResponse = model.RollbackResponse
type Proposal = model.Proposal
type ProposalModels = model.ProposalModels
type ProposalProviders = model.ProposalProviders
type SourceExtraction = model.SourceExtraction
type LineRange = model.LineRange
type MergePlan = model.MergePlan
type Insertion = model.Insertion
type CoverageReport = model.CoverageReport
type MergeJudge = model.MergeJudge
type ProgressFunc = model.ProgressFunc
