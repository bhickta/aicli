package inbox

import (
	"fmt"
	"strings"
)

func normalizeClaims(claims []InboxClaim) []InboxClaim {
	out := make([]InboxClaim, 0, len(claims))
	seen := map[string]int{}
	for i, claim := range claims {
		claim.Text = strings.TrimSpace(claim.Text)
		if claim.Text == "" {
			continue
		}
		claim.ID = strings.TrimSpace(claim.ID)
		if claim.ID == "" {
			claim.ID = fmt.Sprintf("c%d", i+1)
		}
		if count := seen[claim.ID]; count > 0 {
			claim.ID = fmt.Sprintf("%s-%d", claim.ID, count+1)
		}
		seen[claim.ID]++
		out = append(out, claim)
	}
	return out
}

func destinationClaimIDs(destination inboxDestinationAssignment) []string {
	ids := make([]string, 0, len(destination.ClaimIDs)+len(destination.Ledger))
	seen := map[string]bool{}
	for _, id := range destination.ClaimIDs {
		id = strings.TrimSpace(id)
		if id != "" && !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	for _, item := range destination.Ledger {
		id := strings.TrimSpace(item.ClaimID)
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

func normalizeRouteLedger(ledger []InboxClaimLedger, destinationPath string, claims []InboxClaim) []InboxClaimLedger {
	claimSet := claimIDSet(claims)
	out := make([]InboxClaimLedger, 0, len(ledger))
	for _, item := range ledger {
		item.ClaimID = strings.TrimSpace(item.ClaimID)
		if !claimSet[item.ClaimID] {
			continue
		}
		out = append(out, normalizeLedgerItem(item, destinationPath))
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

func pendingLedgerForClaims(claims []InboxClaim, reason string) []InboxClaimLedger {
	out := make([]InboxClaimLedger, 0, len(claims))
	for _, claim := range claims {
		out = append(out, InboxClaimLedger{ClaimID: claim.ID, Status: claimStatusPending, Reason: reason})
	}
	return out
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

func mergeJudgePassed(judge MergeJudge, threshold float64) bool {
	return strings.EqualFold(judge.Verdict, "pass") &&
		judge.Score >= threshold &&
		len(judge.MissingFacts) == 0 &&
		len(judge.UnsupportedAdditions) == 0
}
