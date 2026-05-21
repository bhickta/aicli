package prompt

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/bhickta/aicli/internal/workflow/zettel/model"
)

const fallbackShorthandPrompt = `**ROLE & GOAL**
* **Your Role:** Expert AI Data Compressor and Logic Synthesizer.
* **Primary Objective:** Transform source text into Extreme Shorthand with maximum information density.

**CORE PRINCIPLES**
1. Zero Information Loss: preserve every entity, statistic, date, definition, technical term, list item, qualifier, and analytical relation.
2. Extreme Conciseness: remove grammar/filler; use symbols like ->, =, !=, +, vs, ^, v, ~.
3. Strict Content Filter: keep governance facts, policy decisions, official statements, and verifiable events; exclude gossip, rumors, speculation, personal attacks, and subjective opinion.

**OUTPUT FORMAT**
* Use hyphen bullets.
* Bold primary keyword/subject.
* Use :: or -> to separate term from data.
* English only.`

func LoadShorthandPrompt(options model.Options) string {
	if strings.EqualFold(strings.TrimSpace(options.ShorthandPromptPath), "builtin") {
		return fallbackShorthandPrompt
	}
	content, err := readShorthandPrompt(options.ShorthandPromptPath)
	if err != nil {
		return fallbackShorthandPrompt
	}
	content = dedupePromptTemplate(content)
	content = strings.ReplaceAll(content, "{{CUSTOM_INSTRUCTION_SECTION}}", "")
	content = strings.ReplaceAll(content, "{{LANGUAGE_INSTRUCTIONS_BLOCK}}", "Final output language: English only.")
	content = strings.ReplaceAll(content, "{{SOURCE_TEXT}}", "{{DESTINATION_AND_SOURCE_CLAIMS}}")
	content = strings.TrimSpace(content)
	if content == "" {
		return fallbackShorthandPrompt
	}
	return content
}

func readShorthandPrompt(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("prompt path is required")
	}
	data, err := os.ReadFile(path)
	if err == nil {
		return string(data), nil
	}
	if filepath.IsAbs(path) {
		return "", err
	}
	cwd, cwdErr := os.Getwd()
	if cwdErr != nil {
		return "", err
	}
	data, err = os.ReadFile(filepath.Join(cwd, path))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func dedupePromptTemplate(content string) string {
	const marker = "**ROLE & GOAL**"
	first := strings.Index(content, marker)
	if first < 0 {
		return content
	}
	second := strings.Index(content[first+len(marker):], marker)
	if second < 0 {
		return content
	}
	second += first + len(marker)
	left := strings.TrimSpace(content[first:second])
	right := strings.TrimSpace(content[second:])
	if left == right {
		return left
	}
	return content
}
