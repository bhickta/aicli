package inbox

import (
	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/workflow/zettel/indexer"
	"github.com/bhickta/aicli/internal/workflow/zettel/model"
	"github.com/bhickta/aicli/internal/workflow/zettel/vaultfs"
)

const (
	StatusProcessed = "processed"
	StatusPartial   = "partial"
	StatusPending   = "pending"
	StatusFailed    = "failed"

	inboxStatusProcessed = StatusProcessed
	inboxStatusPartial   = StatusPartial
	inboxStatusPending   = StatusPending
	inboxStatusFailed    = StatusFailed

	claimStatusMerged  = "merged"
	claimStatusDeduped = "deduped"
	claimStatusPending = "pending"
)

type Options = model.Options
type InboxMergeRequest = model.InboxMergeRequest
type InboxMergeResponse = model.InboxMergeResponse
type InboxSourceResult = model.InboxSourceResult
type InboxClaim = model.InboxClaim
type InboxClaimLedger = model.InboxClaimLedger
type InboxDestinationDiff = model.InboxDestinationDiff
type ProgressFunc = model.ProgressFunc

type vault = vaultfs.Vault
type scoredCandidate = indexer.ScoredCandidate

type Runner struct {
	mergeProvider     provider.Provider
	embeddingProvider provider.Provider
}

func New(
	mergeProvider provider.Provider,
	embeddingProvider provider.Provider,
) Runner {
	return Runner{
		mergeProvider:     mergeProvider,
		embeddingProvider: embeddingProvider,
	}
}
