package zettel

import (
	"regexp"
	"sort"
	"strings"
)

var (
	linkPattern    = regexp.MustCompile(`\[\[([^\]|#]+)(?:[#|][^\]]*)?]]`)
	tagPattern     = regexp.MustCompile(`(^|\s)#([A-Za-z0-9_/-]+)`)
	datePattern    = regexp.MustCompile(`(?i)\b(?:\d{1,2}[/-]\d{1,2}[/-]\d{2,4}|\d{4}-\d{2}-\d{2}|(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Sept|Oct|Nov|Dec)[a-z]*\s+\d{1,2},?\s+\d{4})\b`)
	numberPattern  = regexp.MustCompile(`\b\d+(?:[.,]\d+)*(?:\.\d+)?(?:%|[a-zA-Z]+)?\b`)
	headingPattern = regexp.MustCompile(`(?m)^#{1,6}\s+(.+)$`)
)

type traceItems struct {
	Links       []string
	Tags        []string
	Dates       []string
	Numbers     []string
	Headings    []string
	UniqueLines []string
}

func buildCoverageReport(sourceContent string, finalContent string) CoverageReport {
	trace := extractTraceItems(sourceContent)
	missingLinks := missing(trace.Links, finalContent)
	missingTags := missing(trace.Tags, finalContent)
	missingDates := missing(trace.Dates, finalContent)
	missingNumbers := missing(trace.Numbers, finalContent)
	missingHeadings := missing(trace.Headings, finalContent)
	missingUniqueLines := missing(trace.UniqueLines, finalContent)
	if len(missingUniqueLines) > 200 {
		missingUniqueLines = missingUniqueLines[:200]
	}
	totalRequired := len(trace.Links) + len(trace.Tags) + len(trace.Dates) + len(trace.Numbers) + len(trace.Headings)
	missingRequired := len(missingLinks) + len(missingTags) + len(missingDates) + len(missingNumbers) + len(missingHeadings)
	score := 1.0
	if totalRequired > 0 {
		score = float64(totalRequired-missingRequired) / float64(totalRequired)
	}
	if score < 0 {
		score = 0
	}
	return CoverageReport{
		Score:                score,
		RequiredMissingCount: missingRequired,
		MissingLinks:         missingLinks,
		MissingTags:          missingTags,
		MissingDates:         missingDates,
		MissingNumbers:       missingNumbers,
		MissingHeadings:      missingHeadings,
		MissingUniqueLines:   missingUniqueLines,
	}
}

func extractTraceItems(text string) traceItems {
	var links []string
	for _, match := range linkPattern.FindAllStringSubmatch(text, -1) {
		links = append(links, match[1])
	}
	var tags []string
	for _, match := range tagPattern.FindAllStringSubmatch(text, -1) {
		tags = append(tags, "#"+match[2])
	}
	var headings []string
	for _, match := range headingPattern.FindAllStringSubmatch(text, -1) {
		headings = append(headings, strings.TrimSpace(match[1]))
	}
	var uniqueLines []string
	for _, line := range splitLines(text) {
		cleaned := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(line, "- "), "* "), "+ "))
		if len(cleaned) >= 28 && !strings.HasPrefix(cleaned, "```") && !strings.HasPrefix(cleaned, "#") {
			uniqueLines = append(uniqueLines, cleaned)
		}
	}
	return traceItems{
		Links:       uniqueStrings(links),
		Tags:        uniqueStrings(tags),
		Dates:       uniqueStrings(datePattern.FindAllString(text, -1)),
		Numbers:     uniqueStrings(numberPattern.FindAllString(text, -1)),
		Headings:    uniqueStrings(headings),
		UniqueLines: uniqueStrings(uniqueLines),
	}
}

func missing(required []string, finalContent string) []string {
	haystack := strings.ToLower(finalContent)
	var out []string
	for _, item := range required {
		if !strings.Contains(haystack, strings.ToLower(item)) {
			out = append(out, item)
		}
	}
	return out
}

func uniqueStrings(values []string) []string {
	seen := map[string]string{}
	for _, value := range values {
		cleaned := strings.TrimSpace(value)
		if cleaned == "" {
			continue
		}
		key := strings.ToLower(cleaned)
		if _, ok := seen[key]; !ok {
			seen[key] = cleaned
		}
	}
	out := make([]string, 0, len(seen))
	for _, value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
