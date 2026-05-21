package inbox

import (
	"path/filepath"
	"regexp"
	"strings"
)

var inboxTopicTokenPattern = regexp.MustCompile(`[a-z0-9]+`)

func destinationRouteFits(path string, destination inboxDestinationAssignment, claims []InboxClaim, item InboxClaimLedger) bool {
	destinationTokens := strongDestinationTokens(path)
	if len(destinationTokens) == 0 {
		return true
	}
	routeText := destinationRouteText(destination, claims, item)
	if strings.TrimSpace(routeText) == "" {
		return false
	}
	routeTokens := normalizedTopicTokens(routeText)
	for _, destinationToken := range destinationTokens {
		for _, routeToken := range routeTokens {
			if relatedTopicTokens(destinationToken, routeToken) {
				return true
			}
		}
	}
	return false
}

func strongDestinationTokens(path string) []string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	tokens := normalizedTopicTokens(base)
	strong := make([]string, 0, len(tokens))
	seen := map[string]bool{}
	for _, token := range tokens {
		if len(token) < 4 || genericDestinationToken(token) || seen[token] {
			continue
		}
		strong = append(strong, token)
		seen[token] = true
	}
	return strong
}

func destinationRouteText(destination inboxDestinationAssignment, claims []InboxClaim, item InboxClaimLedger) string {
	parts := []string{item.Evidence, item.Reason, destination.Reason}
	for _, claim := range claims {
		if claim.ID == item.ClaimID {
			parts = append(parts, claim.Text, claim.Source)
			break
		}
	}
	for _, action := range destination.Actions {
		claimID := strings.TrimSpace(action.ClaimID)
		if claimID != "" && claimID != item.ClaimID {
			continue
		}
		parts = append(parts, action.Anchor, strings.Join(destinationActionLines(action), "\n"), action.Reason)
	}
	return strings.Join(parts, "\n")
}

func destinationContainsClaim(destinationContent string, claims []InboxClaim, item InboxClaimLedger) bool {
	if strings.TrimSpace(item.Evidence) != "" && strings.Contains(strings.ToLower(destinationContent), strings.ToLower(strings.TrimSpace(item.Evidence))) {
		return true
	}
	claimText := ""
	for _, claim := range claims {
		if claim.ID == item.ClaimID {
			claimText = claim.Text
			break
		}
	}
	claimTokens := meaningfulClaimTokens(claimText)
	if len(claimTokens) == 0 {
		return false
	}
	destinationTokens := topicTokenSet(destinationContent)
	matched := 0
	for _, token := range claimTokens {
		if destinationTokens[token] {
			matched++
		}
	}
	if len(claimTokens) <= 5 {
		return matched == len(claimTokens)
	}
	return matched*10 >= len(claimTokens)*7
}

func meaningfulClaimTokens(value string) []string {
	tokens := normalizedTopicTokens(value)
	out := make([]string, 0, len(tokens))
	seen := map[string]bool{}
	for _, token := range tokens {
		if len(token) < 4 || genericClaimToken(token) || seen[token] {
			continue
		}
		out = append(out, token)
		seen[token] = true
	}
	return out
}

func topicTokenSet(value string) map[string]bool {
	out := map[string]bool{}
	for _, token := range normalizedTopicTokens(value) {
		out[token] = true
	}
	return out
}

func normalizedTopicTokens(value string) []string {
	raw := inboxTopicTokenPattern.FindAllString(strings.ToLower(value), -1)
	tokens := make([]string, 0, len(raw))
	for _, token := range raw {
		token = normalizeTopicToken(token)
		if token != "" {
			tokens = append(tokens, token)
		}
	}
	return tokens
}

func normalizeTopicToken(token string) string {
	token = strings.TrimSpace(strings.ToLower(token))
	if len(token) > 4 && strings.HasSuffix(token, "ies") {
		token = strings.TrimSuffix(token, "ies") + "y"
	}
	if len(token) > 3 && strings.HasSuffix(token, "s") {
		token = strings.TrimSuffix(token, "s")
	}
	if token == "economy" {
		return "economic"
	}
	return token
}

func relatedTopicTokens(a string, b string) bool {
	if a == b {
		return true
	}
	if len(a) >= 5 && len(b) >= 5 && (strings.Contains(a, b) || strings.Contains(b, a)) {
		return true
	}
	return false
}

func genericClaimToken(token string) bool {
	if genericDestinationToken(token) {
		return true
	}
	switch token {
	case "through", "include", "includes", "included", "using", "already", "represented",
		"source", "claim", "same", "fact", "tool", "primary", "existing":
		return true
	default:
		return false
	}
}

func genericDestinationToken(token string) bool {
	switch token {
	case "upsc", "prelim", "main", "concept", "economic", "introduction", "intro", "overview",
		"study", "material", "next", "topic", "analysis", "application", "core",
		"foundation", "foundational", "definition", "note", "notes", "existing",
		"misc", "miscellaneous", "part", "chapter", "class":
		return true
	default:
		return false
	}
}
