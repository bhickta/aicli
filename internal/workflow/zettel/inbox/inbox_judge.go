package inbox

import (
	"strings"
)

func parseInboxDestinationJudgement(text string) (inboxDestinationJudgement, bool) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	var judgement inboxDestinationJudgement
	seen := map[string]bool{}
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		upper := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(upper, "PENDING:"):
			reason := strings.TrimSpace(line[len("PENDING:"):])
			if reason == "" {
				reason = "judge found no safe destination"
			}
			judgement.PendingReason = reason
		case strings.HasPrefix(upper, "TARGET ") || strings.HasPrefix(upper, "TARGET:"):
			path := strings.TrimSpace(strings.TrimPrefix(line, line[:len("TARGET")]))
			path = strings.TrimSpace(strings.TrimPrefix(path, ":"))
			path = strings.Trim(path, "`\"")
			if path != "" && !seen[path] {
				judgement.Targets = append(judgement.Targets, path)
				seen[path] = true
			}
		}
	}
	if judgement.PendingReason != "" {
		return inboxDestinationJudgement{PendingReason: judgement.PendingReason}, true
	}
	return judgement, len(judgement.Targets) > 0
}

func selectJudgedCandidates(candidates []scoredCandidate, targets []string) ([]scoredCandidate, []string) {
	byPath := map[string]scoredCandidate{}
	for _, candidate := range candidates {
		byPath[strings.TrimSpace(candidate.Path)] = candidate
	}
	selected := make([]scoredCandidate, 0, len(targets))
	rejected := []string{}
	seen := map[string]bool{}
	for _, target := range targets {
		path := strings.TrimSpace(target)
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		candidate, ok := byPath[path]
		if !ok {
			rejected = append(rejected, path)
			continue
		}
		selected = append(selected, candidate)
	}
	return selected, rejected
}

func pendingInboxDecision(sourcePath string, reason string) inboxDestinationDecision {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "no safe destination found"
	}
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

func parseInboxValidationResult(text string) inboxValidationResult {
	line := firstNonEmptyLine(text)
	upper := strings.ToUpper(line)
	switch {
	case upper == "PASS":
		return inboxValidationResult{OK: true, Pass: true}
	case strings.HasPrefix(upper, "PASS ") || strings.HasPrefix(upper, "PASS:"):
		return inboxValidationResult{OK: true, Pass: true}
	case strings.HasPrefix(upper, "FAIL:"):
		reason := strings.TrimSpace(line[len("FAIL:"):])
		if reason == "" {
			reason = "validator rejected final notes"
		}
		return inboxValidationResult{OK: true, Reason: reason}
	case upper == "FAIL":
		return inboxValidationResult{OK: true, Reason: "validator rejected final notes"}
	default:
		return inboxValidationResult{}
	}
}

func firstNonEmptyLine(text string) string {
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}
