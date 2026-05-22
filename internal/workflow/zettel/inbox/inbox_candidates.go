package inbox

import "strings"

func constrainDecisionToCandidates(decision inboxDestinationDecision, candidates []scoredCandidate) inboxDestinationDecision {
	allowed := map[string]bool{}
	for _, candidate := range candidates {
		allowed[candidate.Path] = true
	}
	out := decision
	out.Destinations = nil
	out.Pending = append([]InboxClaimLedger{}, decision.Pending...)
	for _, destination := range decision.Destinations {
		path := strings.TrimSpace(destination.Path)
		if allowed[path] {
			out.Destinations = append(out.Destinations, destination)
			continue
		}
		for _, claimID := range destinationClaimIDs(destination) {
			out.Pending = append(out.Pending, InboxClaimLedger{
				ClaimID:         claimID,
				Status:          claimStatusPending,
				DestinationPath: path,
				Reason:          "destination was not in current semantic candidate set",
			})
		}
	}
	return out
}

func appendInboxCandidatePath(candidates []scoredCandidate, path string) []scoredCandidate {
	path = strings.TrimSpace(path)
	if path == "" {
		return candidates
	}
	for _, candidate := range candidates {
		if candidate.Path == path {
			return candidates
		}
	}
	return append(candidates, scoredCandidate{Path: path, Similarity: 0})
}
