package inbox

import (
	"fmt"
	"strings"

	"github.com/bhickta/aicli/internal/workflow/zettel/notetext"
)

const finalNoteClaimID = "source"

func parseInboxFinalNotes(sourcePath string, text string) (inboxDestinationDecision, bool) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	var decision inboxDestinationDecision
	var pendingReason string
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(strings.ToUpper(line), "PENDING:") {
			pendingReason = strings.TrimSpace(line[len("PENDING:"):])
			continue
		}
		path, ok := parseBeginNoteLine(line)
		if !ok {
			continue
		}
		var body []string
		closed := false
		i++
		for ; i < len(lines); i++ {
			if strings.EqualFold(strings.TrimSpace(lines[i]), "END_NOTE") {
				closed = true
				break
			}
			body = append(body, lines[i])
		}
		if !closed {
			return inboxDestinationDecision{}, false
		}
		markdown := notetext.EnsureTrailingNewline(strings.Trim(strings.Join(body, "\n"), "\n"))
		if path == "" || strings.TrimSpace(markdown) == "" {
			continue
		}
		decision.Destinations = append(decision.Destinations, inboxDestinationAssignment{
			Path:       path,
			ClaimIDs:   []string{finalNoteClaimID},
			FinalNote:  markdown,
			Confidence: 1,
			Reason:     "final note returned by merge model",
		})
	}
	if len(decision.Destinations) == 0 && pendingReason == "" {
		return inboxDestinationDecision{}, false
	}
	decision.Claims = []InboxClaim{{
		ID:     finalNoteClaimID,
		Text:   "Full source note must be represented losslessly in final destination note(s).",
		Source: sourcePath,
	}}
	if pendingReason != "" {
		decision.Pending = append(decision.Pending, InboxClaimLedger{
			ClaimID: finalNoteClaimID,
			Status:  claimStatusPending,
			Reason:  pendingReason,
		})
	}
	return decision, true
}

func parseBeginNoteLine(line string) (string, bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 || !strings.EqualFold(strings.TrimSuffix(fields[0], ":"), "BEGIN_NOTE") {
		return "", false
	}
	path := strings.TrimSpace(strings.TrimPrefix(line, fields[0]))
	path = strings.TrimSpace(strings.TrimPrefix(path, ":"))
	path = strings.Trim(path, "`\"")
	return path, true
}

func pendingInboxDecision(sourcePath string, reason string) inboxDestinationDecision {
	return inboxDestinationDecision{
		Claims: []InboxClaim{{
			ID:     finalNoteClaimID,
			Text:   "Full source note must be represented losslessly in final destination note(s).",
			Source: sourcePath,
		}},
		Pending: []InboxClaimLedger{{
			ClaimID: finalNoteClaimID,
			Status:  claimStatusPending,
			Reason:  reason,
		}},
	}
}

func ensureInboxDestinationsCurrent(v vault, options Options, decision inboxDestinationDecision, candidates []scoredCandidate) error {
	candidateContent := make(map[string]string, len(candidates))
	for _, candidate := range candidates {
		candidateContent[candidate.Path] = candidate.Content
	}
	for _, destination := range decision.Destinations {
		path := strings.TrimSpace(destination.Path)
		before, ok := candidateContent[path]
		if !ok {
			continue
		}
		current, err := readDestinationNote(v, options, path)
		if err != nil {
			return err
		}
		if current != before {
			return fmt.Errorf("destination changed during parallel run: %s; retry this source", path)
		}
	}
	return nil
}

func synthesizeFinalNoteLedger(path string, changed bool) InboxClaimLedger {
	status := claimStatusMerged
	reason := "final note preserves source and destination facts"
	if !changed {
		status = claimStatusDeduped
		reason = "final note is unchanged; source appears already represented"
	}
	return InboxClaimLedger{
		ClaimID:         finalNoteClaimID,
		Status:          status,
		DestinationPath: path,
		Evidence:        fmt.Sprintf("complete final note for %s", path),
		Reason:          reason,
	}
}
