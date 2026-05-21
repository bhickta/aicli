package manual

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

func readActive(options Options, path string) (vault, string, string, error) {
	v, err := newVault(options.VaultPath)
	if err != nil {
		return vault{}, "", "", err
	}
	abs, err := v.NotePath(path, options)
	if err != nil {
		return vault{}, "", "", err
	}
	rel, err := v.Rel(abs)
	if err != nil {
		return vault{}, "", "", err
	}
	content, err := os.ReadFile(abs)
	if err != nil {
		return vault{}, "", "", fmt.Errorf("read active note: %w", err)
	}
	return v, rel, string(content), nil
}

func buildExtractions(v vault, options Options, selections []Selection) ([]SourceExtraction, string, error) {
	extractions := make([]SourceExtraction, 0, len(selections))
	sourceBlocks := make([]string, 0, len(selections))
	for _, selection := range selections {
		abs, err := v.NotePath(selection.Path, options)
		if err != nil {
			return nil, "", err
		}
		rel, err := v.Rel(abs)
		if err != nil {
			return nil, "", err
		}
		contentBytes, err := os.ReadFile(abs)
		if err != nil {
			return nil, "", fmt.Errorf("read source note %s: %w", rel, err)
		}
		content := string(contentBytes)
		ranges, err := normalizeRanges(selection.SourceLineRanges, content, len(splitLines(content)))
		if err != nil {
			return nil, "", fmt.Errorf("source note %s: %w", rel, err)
		}
		extracted := extractLineRanges(content, ranges)
		if strings.TrimSpace(extracted) == "" {
			return nil, "", fmt.Errorf("source note %s selected ranges are empty", rel)
		}
		extractions = append(extractions, SourceExtraction{
			Path:              rel,
			OriginalHash:      hashText(content),
			SourceLineRanges:  ranges,
			ExtractedMarkdown: extracted,
		})
		sourceBlocks = append(sourceBlocks, extracted)
	}
	return extractions, strings.Join(sourceBlocks, "\n\n--- SOURCE BREAK ---\n\n"), nil
}

func normalizeRanges(ranges []LineRange, content string, upperLine int) ([]LineRange, error) {
	if len(ranges) == 0 {
		return nil, errors.New("source line ranges are required")
	}
	totalLines := len(splitLines(content))
	if upperLine <= 0 || upperLine > totalLines {
		upperLine = totalLines
	}
	normalized := mergeLineRanges(ranges)
	for _, r := range normalized {
		if r.StartLine < 1 || r.EndLine < r.StartLine || r.EndLine > upperLine {
			return nil, fmt.Errorf("invalid line range %d-%d", r.StartLine, r.EndLine)
		}
	}
	return normalized, nil
}
