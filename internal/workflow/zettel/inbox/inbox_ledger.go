package inbox

import "strings"

func destinationClaimIDs(destination inboxDestinationAssignment) []string {
	ids := make([]string, 0, len(destination.ClaimIDs))
	seen := map[string]bool{}
	for _, id := range destination.ClaimIDs {
		id = strings.TrimSpace(id)
		if id != "" && !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	return ids
}

func claimIDSet(claims []InboxClaim) map[string]bool {
	out := make(map[string]bool, len(claims))
	for _, claim := range claims {
		out[claim.ID] = true
	}
	return out
}

func normalizeLedgerItem(item InboxClaimLedger, destinationPath string) InboxClaimLedger {
	item.ClaimID = strings.TrimSpace(item.ClaimID)
	item.Status = strings.ToLower(strings.TrimSpace(item.Status))
	if item.Status != claimStatusMerged && item.Status != claimStatusDeduped && item.Status != claimStatusPending {
		item.Status = claimStatusPending
		if item.Reason == "" {
			item.Reason = "unknown ledger status"
		}
	}
	if item.DestinationPath == "" && destinationPath != "" {
		item.DestinationPath = destinationPath
	}
	return item
}

func ensureAllClaimsAccounted(claims []InboxClaim, ledger []InboxClaimLedger) []InboxClaimLedger {
	accounted := map[string]bool{}
	mentioned := map[string]bool{}
	for _, item := range ledger {
		if item.ClaimID != "" {
			mentioned[item.ClaimID] = true
		}
		if item.Status == claimStatusMerged || item.Status == claimStatusDeduped {
			accounted[item.ClaimID] = true
		}
	}
	seen := map[string]bool{}
	out := make([]InboxClaimLedger, 0, len(ledger)+len(claims))
	for _, item := range ledger {
		if item.Status == claimStatusPending && accounted[item.ClaimID] {
			continue
		}
		key := item.ClaimID + "\x00" + item.Status + "\x00" + item.DestinationPath
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	for _, claim := range claims {
		if !accounted[claim.ID] && !mentioned[claim.ID] {
			out = append(out, InboxClaimLedger{ClaimID: claim.ID, Status: claimStatusPending, Reason: "claim was not accounted for"})
		}
	}
	return out
}

func countLedgerStatuses(ledger []InboxClaimLedger) (int, int, int) {
	merged := 0
	deduped := 0
	pending := 0
	seenAccounted := map[string]bool{}
	for _, item := range ledger {
		switch item.Status {
		case claimStatusMerged:
			if !seenAccounted[item.ClaimID] {
				merged++
				seenAccounted[item.ClaimID] = true
			}
		case claimStatusDeduped:
			if !seenAccounted[item.ClaimID] {
				deduped++
				seenAccounted[item.ClaimID] = true
			}
		case claimStatusPending:
			pending++
		}
	}
	return merged, deduped, pending
}

func firstPendingReason(ledger []InboxClaimLedger, fallback string) string {
	for _, item := range ledger {
		if item.Status == claimStatusPending && strings.TrimSpace(item.Reason) != "" {
			return item.Reason
		}
	}
	return fallback
}
