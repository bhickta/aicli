package inbox

import (
	"context"
	"regexp"
	"sort"
	"strings"
)

const maxLexicalInboxCandidates = 8

var inboxPhrasePattern = regexp.MustCompile(`\*\*([^*\n]{3,80})\*\*|"([^"\n]{3,120})"|'([^'\n]{3,120})'`)

type lexicalInboxCandidate struct {
	candidate scoredCandidate
	score     int
}

func augmentInboxCandidates(ctx context.Context, v vault, options Options, sourcePath string, sourceContent string, candidates []scoredCandidate) []scoredCandidate {
	phrases := inboxLexicalPhrases(sourceContent)
	if len(phrases) == 0 {
		return candidates
	}
	seen := map[string]bool{}
	for _, candidate := range candidates {
		seen[candidate.Path] = true
	}
	notes, err := v.ScanNotes(options)
	if err != nil {
		return candidates
	}
	matches := make([]lexicalInboxCandidate, 0, maxLexicalInboxCandidates)
	for _, path := range notes {
		select {
		case <-ctx.Done():
			return candidates
		default:
		}
		if path == sourcePath || seen[path] {
			continue
		}
		content, err := readDestinationContent(v, options, path)
		if err != nil {
			continue
		}
		score := inboxLexicalScore(content, phrases)
		if score == 0 {
			continue
		}
		matches = append(matches, lexicalInboxCandidate{
			candidate: scoredCandidate{Path: path, Content: content, Similarity: 1 + float64(score)/100},
			score:     score,
		})
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].score == matches[j].score {
			return matches[i].candidate.Path < matches[j].candidate.Path
		}
		return matches[i].score > matches[j].score
	})
	if len(matches) > maxLexicalInboxCandidates {
		matches = matches[:maxLexicalInboxCandidates]
	}
	out := make([]scoredCandidate, 0, len(candidates)+len(matches))
	out = append(out, candidates...)
	for _, match := range matches {
		out = append(out, match.candidate)
	}
	return out
}

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
				Reason:          "destination was not in current candidate set",
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

func inboxLexicalPhrases(content string) []string {
	seen := map[string]bool{}
	phrases := []string{}
	for _, match := range inboxPhrasePattern.FindAllStringSubmatch(content, -1) {
		for _, group := range match[1:] {
			phrase := normalizeInboxPhrase(group)
			if phrase == "" || seen[phrase] {
				continue
			}
			phrases = append(phrases, phrase)
			seen[phrase] = true
		}
	}
	return phrases
}

func inboxLexicalScore(content string, phrases []string) int {
	normalizedContent := strings.ToLower(content)
	score := 0
	for _, phrase := range phrases {
		if strings.Contains(normalizedContent, phrase) {
			score += 10 + len(strings.Fields(phrase))
		}
	}
	return score
}

func normalizeInboxPhrase(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.Trim(value, ".,;:()[]{}")
	if len(value) < 3 || len(value) > 120 {
		return ""
	}
	if len(strings.Fields(value)) == 1 && len(value) < 6 {
		return ""
	}
	return value
}

func readDestinationContent(v vault, options Options, path string) (string, error) {
	return readDestinationNote(v, options, path)
}
