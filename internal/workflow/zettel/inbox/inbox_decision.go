package inbox

import (
	"strings"

	"github.com/bhickta/aicli/internal/workflow/zettel/notetext"
)

type inboxAppliedDecision struct {
	ledger            []InboxClaimLedger
	destinationBefore map[string]string
	destinationAfter  map[string]string
	destinationWrites map[string]string
	destinationDiffs  []InboxDestinationDiff
	destinationPaths  []string
	rewriteAttempted  bool
}

func materializeInboxDecision(v vault, options Options, decision inboxDestinationDecision, claims []InboxClaim) (inboxAppliedDecision, error) {
	applied := newInboxAppliedDecision()
	claimSet := claimIDSet(claims)
	assigned := map[string]bool{}

	applied.addDecisionPending(decision.Pending, claimSet)
	for _, destination := range decision.Destinations {
		if err := applied.materializeDestination(v, options, destination, claims, claimSet, assigned); err != nil {
			return applied, err
		}
	}

	applied.ledger = ensureAllClaimsAccounted(claims, applied.ledger)
	return applied, nil
}

func newInboxAppliedDecision() inboxAppliedDecision {
	return inboxAppliedDecision{
		destinationBefore: map[string]string{},
		destinationAfter:  map[string]string{},
		destinationWrites: map[string]string{},
	}
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

func (a *inboxAppliedDecision) materializeDestination(v vault, options Options, destination inboxDestinationAssignment, claims []InboxClaim, claimSet map[string]bool, assigned map[string]bool) error {
	path := strings.TrimSpace(destination.Path)
	if path == "" || destination.Confidence < options.ReviewThreshold {
		a.addLowConfidencePending(destination, claimSet, assigned)
		return nil
	}

	ledger := destinationLedger(destination, path, claims)
	if hasLedgerStatus(ledger, claimStatusMerged) {
		a.rewriteAttempted = true
	}
	if len(ledger) == 0 {
		return nil
	}

	if hasLedgerStatus(ledger, claimStatusMerged) || hasLedgerStatus(ledger, claimStatusDeduped) {
		if err := a.ensureDestinationLoaded(v, options, path); err != nil {
			return err
		}
	}
	merged := a.applyImmediateLedger(path, ledger, assigned)
	a.applyMergedLedger(path, destination, merged, assigned)
	return nil
}

func (a *inboxAppliedDecision) addLowConfidencePending(destination inboxDestinationAssignment, claimSet map[string]bool, assigned map[string]bool) {
	for _, id := range destinationClaimIDs(destination) {
		if claimSet[id] && !assigned[id] {
			a.ledger = append(a.ledger, InboxClaimLedger{ClaimID: id, Status: claimStatusPending, Reason: "destination confidence below threshold"})
		}
	}
}

func (a *inboxAppliedDecision) ensureDestinationLoaded(v vault, options Options, path string) error {
	if _, ok := a.destinationBefore[path]; ok {
		return nil
	}
	content, err := readDestinationNote(v, options, path)
	if err != nil {
		return err
	}
	a.destinationBefore[path] = content
	a.destinationAfter[path] = content
	return nil
}

func (a *inboxAppliedDecision) applyImmediateLedger(path string, ledger []InboxClaimLedger, assigned map[string]bool) []InboxClaimLedger {
	merged := make([]InboxClaimLedger, 0, len(ledger))
	for _, item := range ledger {
		if assigned[item.ClaimID] {
			continue
		}
		switch item.Status {
		case claimStatusMerged:
			merged = append(merged, item)
		case claimStatusDeduped:
			a.ledger = append(a.ledger, item)
			a.destinationPaths = appendUniquePath(a.destinationPaths, path)
			assigned[item.ClaimID] = true
		default:
			a.ledger = append(a.ledger, item)
		}
	}
	return merged
}

func (a *inboxAppliedDecision) applyMergedLedger(path string, destination inboxDestinationAssignment, merged []InboxClaimLedger, assigned map[string]bool) {
	if len(merged) == 0 {
		return
	}
	if len(destination.Actions) > 0 {
		a.applyMergedActions(path, destination.Actions, merged, assigned)
		return
	}
	a.ledger = append(a.ledger, pendingLedgerForLedger(merged, "destination missing actions for merged claim")...)
}

func (a *inboxAppliedDecision) applyMergedActions(path string, actions []inboxDestinationAction, merged []InboxClaimLedger, assigned map[string]bool) {
	grouped := groupDestinationActions(actions, merged)
	currentLines := notetext.SplitLines(a.destinationAfter[path])
	changed := false
	for _, item := range merged {
		if assigned[item.ClaimID] {
			continue
		}
		nextLines, actionChanged, represented, reason := applyClaimDestinationActions(currentLines, grouped[item.ClaimID])
		if reason != "" {
			a.ledger = append(a.ledger, pendingLedgerForLedger([]InboxClaimLedger{item}, reason)...)
			continue
		}
		if !represented {
			a.ledger = append(a.ledger, pendingLedgerForLedger([]InboxClaimLedger{item}, "destination actions did not change for merged claim")...)
			continue
		}
		currentLines = nextLines
		changed = changed || actionChanged
		if actionChanged {
			a.ledger = append(a.ledger, item)
		} else {
			deduped := item
			deduped.Status = claimStatusDeduped
			if deduped.Reason == "" {
				deduped.Reason = "destination already contains action lines"
			}
			a.ledger = append(a.ledger, deduped)
		}
		assigned[item.ClaimID] = true
		a.destinationPaths = appendUniquePath(a.destinationPaths, path)
	}
	if !changed {
		return
	}

	before := a.destinationBefore[path]
	after := notetext.EnsureTrailingNewline(strings.Join(currentLines, "\n"))
	a.destinationAfter[path] = after
	a.destinationWrites[path] = after
	a.destinationPaths = appendUniquePath(a.destinationPaths, path)
	a.destinationDiffs = append(a.destinationDiffs, InboxDestinationDiff{
		Path:    path,
		Before:  before,
		After:   after,
		Diff:    notetext.SimpleMarkdownDiff(before, after),
		Created: false,
	})
}

func destinationLedger(destination inboxDestinationAssignment, path string, claims []InboxClaim) []InboxClaimLedger {
	ledger := normalizeRouteLedger(destination.Ledger, path, claims)
	claimSet := claimIDSet(claims)
	mentioned := map[string]bool{}
	for _, item := range ledger {
		mentioned[item.ClaimID] = true
	}
	for _, id := range destination.ClaimIDs {
		id = strings.TrimSpace(id)
		if !claimSet[id] || mentioned[id] {
			continue
		}
		reason := strings.TrimSpace(destination.Reason)
		if reason == "" {
			reason = "destination selected for claim"
		}
		ledger = append(ledger, InboxClaimLedger{
			ClaimID:         id,
			Status:          claimStatusMerged,
			DestinationPath: path,
			Reason:          reason,
		})
		mentioned[id] = true
	}
	for _, action := range destination.Actions {
		id := strings.TrimSpace(action.ClaimID)
		if !claimSet[id] || mentioned[id] {
			continue
		}
		reason := strings.TrimSpace(action.Reason)
		if reason == "" {
			reason = strings.TrimSpace(destination.Reason)
		}
		if reason == "" {
			reason = "destination action selected for claim"
		}
		ledger = append(ledger, InboxClaimLedger{
			ClaimID:         id,
			Status:          claimStatusMerged,
			DestinationPath: path,
			Reason:          reason,
		})
		mentioned[id] = true
	}
	return ledger
}

func pendingLedgerForLedger(ledger []InboxClaimLedger, reason string) []InboxClaimLedger {
	out := make([]InboxClaimLedger, 0, len(ledger))
	for _, item := range ledger {
		out = append(out, InboxClaimLedger{
			ClaimID:         item.ClaimID,
			Status:          claimStatusPending,
			DestinationPath: item.DestinationPath,
			Evidence:        item.Evidence,
			Reason:          reason,
		})
	}
	return out
}

func groupDestinationActions(actions []inboxDestinationAction, merged []InboxClaimLedger) map[string][]inboxDestinationAction {
	grouped := map[string][]inboxDestinationAction{}
	defaultClaimID := ""
	if len(merged) == 1 {
		defaultClaimID = merged[0].ClaimID
	}
	for _, action := range actions {
		claimID := strings.TrimSpace(action.ClaimID)
		if claimID == "" {
			claimID = defaultClaimID
		}
		if claimID == "" {
			continue
		}
		grouped[claimID] = append(grouped[claimID], action)
	}
	return grouped
}

func applyClaimDestinationActions(lines []string, actions []inboxDestinationAction) ([]string, bool, bool, string) {
	if len(actions) == 0 {
		return lines, false, false, "destination missing actions for merged claim"
	}
	next := append([]string{}, lines...)
	changed := false
	represented := false
	for _, action := range actions {
		var actionChanged bool
		var actionRepresented bool
		var reason string
		next, actionChanged, actionRepresented, reason = applyDestinationAction(next, action)
		if reason != "" {
			return lines, false, false, reason
		}
		changed = changed || actionChanged
		represented = represented || actionRepresented
	}
	return next, changed, represented, ""
}

func applyDestinationAction(lines []string, action inboxDestinationAction) ([]string, bool, bool, string) {
	actionType := normalizeDestinationActionType(action.Type)
	insertLines := destinationActionLines(action)
	if actionType == "pending" {
		return lines, false, false, destinationActionReason(action, "destination action marked pending")
	}
	if len(insertLines) == 0 {
		return lines, false, false, "destination action missing lines"
	}
	if destinationAlreadyContainsLines(lines, insertLines) {
		return lines, false, true, ""
	}

	index, reason := destinationActionIndex(lines, actionType, action)
	if reason != "" {
		return lines, false, false, reason
	}
	out := make([]string, 0, len(lines)+len(insertLines))
	out = append(out, lines[:index]...)
	out = append(out, insertLines...)
	out = append(out, lines[index:]...)
	return out, true, true, ""
}

func normalizeDestinationActionType(actionType string) string {
	switch strings.ToLower(strings.TrimSpace(actionType)) {
	case "insert_after", "insert_after_line", "insert_after_exact_line", "add_after":
		return "insert_after"
	case "insert_before", "insert_before_line", "insert_before_exact_line", "add_before":
		return "insert_before"
	case "append", "append_to_end", "add_to_end":
		return "append_to_end"
	case "pending":
		return "pending"
	default:
		return ""
	}
}

func destinationActionLines(action inboxDestinationAction) []string {
	raw := append([]string{}, action.Lines...)
	if strings.TrimSpace(action.Line) != "" {
		raw = append(raw, action.Line)
	}
	lines := make([]string, 0, len(raw))
	for _, item := range raw {
		for _, line := range notetext.SplitLines(strings.Trim(item, "\n")) {
			if strings.TrimSpace(line) != "" {
				lines = append(lines, line)
			}
		}
	}
	return lines
}

func destinationActionIndex(lines []string, actionType string, action inboxDestinationAction) (int, string) {
	switch actionType {
	case "append_to_end":
		return len(lines), ""
	case "insert_after", "insert_before":
	default:
		return 0, "unknown destination action type"
	}

	lineIndex, reason := destinationAnchorIndex(lines, action)
	if reason != "" {
		return 0, reason
	}
	if actionType == "insert_after" {
		return lineIndex + 1, ""
	}
	return lineIndex, ""
}

func destinationAnchorIndex(lines []string, action inboxDestinationAction) (int, string) {
	anchor := strings.TrimRight(action.Anchor, "\n")
	if action.LineNumber > 0 {
		index := action.LineNumber - 1
		if index < 0 || index >= len(lines) {
			return 0, "destination action line number outside destination"
		}
		if anchor != "" && lines[index] != anchor {
			return 0, "destination action anchor does not match line number"
		}
		return index, ""
	}
	if anchor == "" {
		return 0, "destination action missing exact anchor"
	}
	found := -1
	for i, line := range lines {
		if line != anchor {
			continue
		}
		if found != -1 {
			return 0, "destination action anchor matched multiple lines"
		}
		found = i
	}
	if found == -1 {
		return 0, "destination action anchor not found"
	}
	return found, ""
}

func destinationAlreadyContainsLines(lines []string, insertLines []string) bool {
	if len(insertLines) == 0 || len(insertLines) > len(lines) {
		return false
	}
	for i := 0; i <= len(lines)-len(insertLines); i++ {
		match := true
		for j, line := range insertLines {
			if lines[i+j] != line {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func destinationActionReason(action inboxDestinationAction, fallback string) string {
	if strings.TrimSpace(action.Reason) != "" {
		return action.Reason
	}
	return fallback
}

func hasLedgerStatus(ledger []InboxClaimLedger, status string) bool {
	for _, item := range ledger {
		if item.Status == status {
			return true
		}
	}
	return false
}
