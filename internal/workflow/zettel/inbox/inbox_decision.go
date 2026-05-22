package inbox

import (
	"errors"
	"os"
	"strings"

	"github.com/bhickta/aicli/internal/workflow/zettel/notetext"
)

type inboxAppliedDecision struct {
	ledger            []InboxClaimLedger
	destinationBefore map[string]string
	destinationWrites map[string]string
	destinationDiffs  []InboxDestinationDiff
	destinationPaths  []string
}

func materializeInboxDecision(v vault, options Options, decision inboxDestinationDecision, claims []InboxClaim) (inboxAppliedDecision, error) {
	applied := inboxAppliedDecision{
		destinationBefore: map[string]string{},
		destinationWrites: map[string]string{},
	}
	claimSet := claimIDSet(claims)

	applied.addDecisionPending(decision.Pending, claimSet)
	for _, destination := range decision.Destinations {
		if err := applied.materializeFinalNoteDestination(v, options, destination, claimSet); err != nil {
			return applied, err
		}
	}

	applied.ledger = ensureAllClaimsAccounted(claims, applied.ledger)
	return applied, nil
}

func (a *inboxAppliedDecision) addDecisionPending(pendingItems []InboxClaimLedger, claimSet map[string]bool) {
	for _, pending := range pendingItems {
		pending = normalizeLedgerItem(pending, "")
		pending.Status = claimStatusPending
		if claimSet[pending.ClaimID] {
			a.ledger = append(a.ledger, pending)
		}
	}
}

func (a *inboxAppliedDecision) materializeFinalNoteDestination(
	v vault,
	options Options,
	destination inboxDestinationAssignment,
	claimSet map[string]bool,
) error {
	path := strings.TrimSpace(destination.Path)
	if path == "" || destination.Confidence < options.ReviewThreshold {
		a.addDestinationPending(destination, claimSet, "destination confidence below threshold")
		return nil
	}
	if strings.TrimSpace(destination.FinalNote) == "" {
		a.addDestinationPending(destination, claimSet, "final note was empty")
		return nil
	}

	before, created, err := loadFinalNoteDestination(v, options, path)
	if err != nil {
		a.addDestinationPending(destination, claimSet, err.Error())
		return nil
	}
	a.destinationBefore[path] = before

	after := notetext.EnsureTrailingNewline(formatFinalNoteMarkdown(before, destination.FinalNote))
	if strings.TrimSpace(after) == "" {
		a.addDestinationPending(destination, claimSet, "final note was empty")
		return nil
	}

	changed := before != after
	if changed {
		a.destinationWrites[path] = after
		a.destinationDiffs = append(a.destinationDiffs, InboxDestinationDiff{
			Path:    path,
			Before:  before,
			After:   after,
			Diff:    notetext.SimpleMarkdownDiff(before, after),
			Created: created,
		})
	}

	for _, claimID := range destinationClaimIDs(destination) {
		if !claimSet[claimID] {
			continue
		}
		item := synthesizeFinalNoteLedger(path, changed)
		item.ClaimID = claimID
		a.ledger = append(a.ledger, item)
		a.destinationPaths = appendUniquePath(a.destinationPaths, path)
	}
	return nil
}

func loadFinalNoteDestination(v vault, options Options, path string) (string, bool, error) {
	content, err := readDestinationNote(v, options, path)
	if err == nil {
		return content, false, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", false, err
	}
	root := strings.Trim(options.RootFolder, "/")
	normalizedPath := strings.Trim(path, "/")
	if root != "" && normalizedPath != root && !strings.HasPrefix(normalizedPath, root+"/") {
		return "", false, errors.New("new final note path must be inside zettelkasten root")
	}
	return "", true, nil
}

func (a *inboxAppliedDecision) addDestinationPending(destination inboxDestinationAssignment, claimSet map[string]bool, reason string) {
	for _, id := range destinationClaimIDs(destination) {
		if claimSet[id] {
			a.ledger = append(a.ledger, InboxClaimLedger{
				ClaimID:         id,
				Status:          claimStatusPending,
				DestinationPath: strings.TrimSpace(destination.Path),
				Reason:          reason,
			})
		}
	}
}
