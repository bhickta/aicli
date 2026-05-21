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

func normalizeInboxAssignments(decision inboxDestinationDecision, claims []InboxClaim, options Options) (map[string][]string, []InboxClaimLedger) {
	claimSet := claimIDSet(claims)
	assignments := map[string][]string{}
	assigned := map[string]bool{}
	ledger := []InboxClaimLedger{}
	for _, pending := range decision.Pending {
		pending.ClaimID = strings.TrimSpace(pending.ClaimID)
		if claimSet[pending.ClaimID] {
			pending.Status = claimStatusPending
			ledger = append(ledger, pending)
		}
	}
	for _, destination := range decision.Destinations {
		path := strings.TrimSpace(destination.Path)
		if path == "" || destination.Confidence < options.ReviewThreshold {
			for _, id := range destinationClaimIDs(destination) {
				if claimSet[id] {
					ledger = append(ledger, InboxClaimLedger{ClaimID: id, Status: claimStatusPending, Reason: "destination confidence below threshold"})
				}
			}
			continue
		}
		if len(destination.Ledger) > 0 {
			for _, item := range normalizeRouteLedger(destination.Ledger, path, claims) {
				if assigned[item.ClaimID] {
					continue
				}
				switch item.Status {
				case claimStatusMerged:
					assignments[path] = append(assignments[path], item.ClaimID)
					assigned[item.ClaimID] = true
				case claimStatusDeduped:
					ledger = append(ledger, item)
					assigned[item.ClaimID] = true
				default:
					ledger = append(ledger, item)
				}
			}
			for _, id := range destination.ClaimIDs {
				id = strings.TrimSpace(id)
				if !claimSet[id] || assigned[id] {
					continue
				}
				assignments[path] = append(assignments[path], id)
				assigned[id] = true
			}
			continue
		}
		for _, id := range destination.ClaimIDs {
			id = strings.TrimSpace(id)
			if !claimSet[id] || assigned[id] {
				continue
			}
			assignments[path] = append(assignments[path], id)
			assigned[id] = true
		}
	}
	return assignments, ledger
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

func selectClaims(claims []InboxClaim, ids []string) []InboxClaim {
	allowed := map[string]bool{}
	for _, id := range ids {
		allowed[id] = true
	}
	out := make([]InboxClaim, 0, len(ids))
	for _, claim := range claims {
		if allowed[claim.ID] {
			out = append(out, claim)
		}
	}
	return out
}

func normalizeRewriteLedger(ledger []InboxClaimLedger, destinationPath string, claims []InboxClaim) []InboxClaimLedger {
	claimSet := claimIDSet(claims)
	out := make([]InboxClaimLedger, 0, len(ledger))
	for _, item := range ledger {
		if !claimSet[item.ClaimID] {
			continue
		}
		out = append(out, normalizeLedgerItem(item, destinationPath))
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

func ledgerHasStatus(ledger []InboxClaimLedger, status string) bool {
	for _, item := range ledger {
		if item.Status == status {
			return true
		}
	}
	return false
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

func appliedLedger(ledger []InboxClaimLedger) []InboxClaimLedger {
	out := make([]InboxClaimLedger, 0, len(ledger))
	for _, item := range ledger {
		if item.Status == claimStatusPending {
			continue
		}
		out = append(out, item)
	}
	return out
}

func appliedClaimsSource(sourcePath string, claims []InboxClaim, ledger []InboxClaimLedger) string {
	applied := map[string]bool{}
	for _, item := range ledger {
		if item.Status != claimStatusPending {
			applied[item.ClaimID] = true
		}
	}
	lines := []string{"PATH: " + sourcePath, "APPLIED CLAIMS:"}
	for _, claim := range claims {
		if !applied[claim.ID] {
			continue
		}
		source := strings.TrimSpace(claim.Source)
		if source == "" {
			lines = append(lines, fmt.Sprintf("- %s: %s", claim.ID, claim.Text))
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s: %s | source: %s", claim.ID, claim.Text, source))
	}
	return strings.Join(lines, "\n")
}
