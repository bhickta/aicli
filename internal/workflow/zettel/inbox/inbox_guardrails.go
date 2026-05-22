package inbox

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	quotedPhrasePattern = regexp.MustCompile(`"([^"\n]{4,})"|'([^'\n]{4,})'`)
	numberAnchorPattern = regexp.MustCompile(`\b\d+(?:\.\d+)?%?\b`)
	badShorthandPattern = regexp.MustCompile(`(?i)\b(sust dev|social dev|sector init|infinite dims|dims)\b`)
	spacePattern        = regexp.MustCompile(`\s+`)
)

func validateInboxDecisionGuardrails(sourceContent string, adoptedPath string, decision inboxDestinationDecision) error {
	combinedFinal := strings.Builder{}
	for _, destination := range decision.Destinations {
		if reason := badFinalNoteStyle(destination.FinalNote); reason != "" {
			return fmt.Errorf("local validation failed for %s: %s", destination.Path, reason)
		}
		if !destinationAllowedBySourceCategory(adoptedPath, destination.Path) {
			return fmt.Errorf("local validation failed for %s: destination folder does not match source category", destination.Path)
		}
		combinedFinal.WriteString(destination.FinalNote)
		combinedFinal.WriteByte('\n')
	}
	if repeated, ok := repeatedLongLineAcrossDestinations(decision.Destinations); ok {
		return fmt.Errorf("local validation failed: repeated long fact across multiple final notes: %s", repeated)
	}
	if missing := missingSourceAnchors(sourceContent, combinedFinal.String()); len(missing) > 0 {
		return fmt.Errorf("local validation failed: source anchors missing from final notes: %s", strings.Join(missing, ", "))
	}
	return nil
}

func badFinalNoteStyle(markdown string) string {
	if strings.Contains(markdown, "::") {
		return "uses double-colon shorthand"
	}
	if match := badShorthandPattern.FindString(markdown); match != "" {
		return "uses cryptic shorthand: " + match
	}
	if snakeTokenPattern.MatchString(markdown) {
		return "uses underscore-separated wording"
	}
	return ""
}

func destinationAllowedBySourceCategory(adoptedPath string, destinationPath string) bool {
	adoptedCategory, ok := firstCategorySegment(adoptedPath)
	if !ok {
		return true
	}
	destinationCategory, ok := firstCategorySegment(destinationPath)
	if !ok {
		return true
	}
	return strings.EqualFold(adoptedCategory, destinationCategory)
}

func firstCategorySegment(path string) (string, bool) {
	parts := pathSegments(path)
	if len(parts) < 3 {
		return "", false
	}
	return parts[1], true
}

func pathSegments(path string) []string {
	path = strings.Trim(strings.ReplaceAll(path, "\\", "/"), "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}

func repeatedLongLineAcrossDestinations(destinations []inboxDestinationAssignment) (string, bool) {
	seen := map[string]string{}
	for _, destination := range destinations {
		for _, line := range strings.Split(destination.FinalNote, "\n") {
			normalized := normalizeFactLine(line)
			if len(normalized) < 80 {
				continue
			}
			if firstPath, ok := seen[normalized]; ok && firstPath != destination.Path {
				return line, true
			}
			seen[normalized] = destination.Path
		}
	}
	return "", false
}

func missingSourceAnchors(sourceContent string, finalContent string) []string {
	finalNormalized := normalizeAnchorText(finalContent)
	anchors := sourceAnchors(sourceContent)
	missing := make([]string, 0, len(anchors))
	for _, anchor := range anchors {
		if !strings.Contains(finalNormalized, normalizeAnchorText(anchor)) {
			missing = append(missing, anchor)
			if len(missing) >= 5 {
				break
			}
		}
	}
	return missing
}

func sourceAnchors(content string) []string {
	seen := map[string]bool{}
	anchors := []string{}
	for _, match := range quotedPhrasePattern.FindAllStringSubmatch(content, -1) {
		value := strings.TrimSpace(match[1])
		if value == "" {
			value = strings.TrimSpace(match[2])
		}
		if value != "" && !seen[value] {
			anchors = append(anchors, value)
			seen[value] = true
		}
	}
	for _, value := range numberAnchorPattern.FindAllString(content, -1) {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			anchors = append(anchors, value)
			seen[value] = true
		}
	}
	return anchors
}

func normalizeFactLine(line string) string {
	line = strings.TrimSpace(line)
	line = strings.TrimLeft(line, "-*#>0123456789. )\t")
	return normalizeAnchorText(line)
}

func normalizeAnchorText(text string) string {
	text = strings.ToLower(text)
	text = strings.NewReplacer(
		"`", "",
		"*", "",
		"_", " ",
		"\"", "",
		"'", "",
	).Replace(text)
	return strings.TrimSpace(spacePattern.ReplaceAllString(text, " "))
}
