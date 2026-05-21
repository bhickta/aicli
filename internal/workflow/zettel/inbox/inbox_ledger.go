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
		if claimSet[pending.ClaimID] {
			pending.Status = claimStatusPending
			ledger = append(ledger, pending)
		}
	}
	for _, destination := range decision.Destinations {
		path := strings.TrimSpace(destination.Path)
		if path == "" || destination.Confidence < options.ReviewThreshold {
			for _, id := range destination.ClaimIDs {
				if claimSet[id] {
					ledger = append(ledger, InboxClaimLedger{ClaimID: id, Status: claimStatusPending, Reason: "destination confidence below threshold"})
				}
			}
			continue
		}
		for _, id := range destination.ClaimIDs {
			if !claimSet[id] || assigned[id] {
				continue
			}
			assignments[path] = append(assignments[path], id)
			assigned[id] = true
		}
	}
	return assignments, ledger
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
		item.Status = strings.ToLower(strings.TrimSpace(item.Status))
		if item.Status != claimStatusMerged && item.Status != claimStatusDeduped && item.Status != claimStatusPending {
			item.Status = claimStatusPending
			if item.Reason == "" {
				item.Reason = "unknown ledger status"
			}
		}
		if item.DestinationPath == "" {
			item.DestinationPath = destinationPath
		}
		out = append(out, item)
	}
	return out
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
