package inbox

import (
	"fmt"
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
	a.applyMergedLedger(path, destination.FinalMarkdown, merged, assigned)
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

func (a *inboxAppliedDecision) applyMergedLedger(path string, finalMarkdown string, merged []InboxClaimLedger, assigned map[string]bool) {
	if len(merged) == 0 {
		return
	}
	if strings.TrimSpace(finalMarkdown) == "" {
		a.ledger = append(a.ledger, pendingLedgerForLedger(merged, "destination missing final markdown")...)
		return
	}

	before := a.destinationBefore[path]
	after := sanitizeGeneratedDestinationMarkdown(before, finalMarkdown)
	if !preservesExistingDestinationLines(before, after) {
		a.ledger = append(a.ledger, pendingLedgerForLedger(merged, "destination rewrite changed existing content")...)
		return
	}
	if reason := validateGeneratedDestinationAdditions(before, after); reason != "" {
		a.ledger = append(a.ledger, pendingLedgerForLedger(merged, reason)...)
		return
	}
	if after == notetext.EnsureTrailingNewline(before) {
		a.ledger = append(a.ledger, pendingLedgerForLedger(merged, "destination final markdown did not change for merged claim")...)
		return
	}

	for _, item := range merged {
		a.ledger = append(a.ledger, item)
		assigned[item.ClaimID] = true
	}
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

func hasLedgerStatus(ledger []InboxClaimLedger, status string) bool {
	for _, item := range ledger {
		if item.Status == status {
			return true
		}
	}
	return false
}

func sanitizeGeneratedDestinationMarkdown(before string, generated string) string {
	if hasLeadingYAMLFrontmatter(before) {
		return notetext.EnsureTrailingNewline(generated)
	}
	return notetext.EnsureTrailingNewline(stripLeadingYAMLFrontmatter(generated))
}

func hasLeadingYAMLFrontmatter(content string) bool {
	first, rest, ok := strings.Cut(content, "\n")
	if strings.TrimSpace(first) != "---" || !ok {
		return false
	}
	for {
		line, remaining, hasMore := strings.Cut(rest, "\n")
		if strings.TrimSpace(line) == "---" {
			return true
		}
		if !hasMore {
			return false
		}
		rest = remaining
	}
}

func stripLeadingYAMLFrontmatter(content string) string {
	first, rest, ok := strings.Cut(content, "\n")
	if strings.TrimSpace(first) != "---" {
		return content
	}
	if !ok {
		return ""
	}
	remaining := rest
	for {
		line, tail, hasMore := strings.Cut(remaining, "\n")
		if strings.TrimSpace(line) == "---" {
			return tail
		}
		if !hasMore {
			return rest
		}
		remaining = tail
	}
}

func preservesExistingDestinationLines(before string, after string) bool {
	beforeLines := notetext.SplitLines(before)
	if len(beforeLines) == 0 {
		return true
	}
	afterLines := notetext.SplitLines(after)
	beforeIndex := 0
	for _, line := range afterLines {
		if line != beforeLines[beforeIndex] {
			continue
		}
		beforeIndex++
		if beforeIndex == len(beforeLines) {
			return true
		}
	}
	return false
}

func validateGeneratedDestinationAdditions(before string, after string) string {
	labels := destinationBulletLabels(notetext.SplitLines(before))
	for _, line := range insertedDestinationLines(notetext.SplitLines(before), notetext.SplitLines(after)) {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, "- ") {
			return "destination inserted non-bullet content"
		}
		if !strings.Contains(trimmed, "**") {
			return "destination inserted non-telegraphic bullet"
		}
		if len([]rune(trimmed)) > 320 {
			return "destination inserted overlong bullet"
		}
		key, label, ok := destinationBulletLabelKey(line)
		if !ok {
			continue
		}
		if labels[key] {
			return "destination adds duplicate bullet label: " + label
		}
		labels[key] = true
	}
	return ""
}

func insertedDestinationLines(beforeLines []string, afterLines []string) []string {
	inserted := []string{}
	beforeIndex := 0
	for _, line := range afterLines {
		if beforeIndex < len(beforeLines) && line == beforeLines[beforeIndex] {
			beforeIndex++
			continue
		}
		inserted = append(inserted, line)
	}
	return inserted
}

func destinationBulletLabels(lines []string) map[string]bool {
	labels := map[string]bool{}
	for _, line := range lines {
		key, _, ok := destinationBulletLabelKey(line)
		if ok {
			labels[key] = true
		}
	}
	return labels
}

func destinationBulletLabelKey(line string) (string, string, bool) {
	level, rest := markdownIndentLevel(line)
	if !strings.HasPrefix(rest, "- **") {
		return "", "", false
	}
	label, _, ok := strings.Cut(strings.TrimPrefix(rest, "- **"), "**")
	label = strings.TrimSpace(label)
	if !ok || label == "" {
		return "", "", false
	}
	normalized := strings.Join(strings.Fields(strings.ToLower(label)), " ")
	return fmt.Sprintf("%d:%s", level, normalized), label, true
}

func markdownIndentLevel(line string) (int, string) {
	width := 0
	for len(line) > 0 {
		switch line[0] {
		case '\t':
			width += 2
			line = line[1:]
		case ' ':
			width++
			line = line[1:]
		default:
			return width / 2, line
		}
	}
	return width / 2, ""
}
