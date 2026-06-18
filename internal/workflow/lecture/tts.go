package lecture

import (
	"errors"
	"fmt"
	"strings"
)

func (s *Service) ttsCommand(req Request, scriptPath string, audioPath string) (string, []string, error) {
	command := strings.TrimSpace(req.TTSCommand)
	if command == "" {
		command = strings.TrimSpace(s.tools.OTSTTS)
	}
	if command == "" {
		return "", nil, errors.New("ots tts command is required")
	}
	template := strings.TrimSpace(req.TTSArgs)
	if template == "" {
		template = strings.TrimSpace(s.tools.OTSTTSArgs)
	}
	if template == "" {
		template = `SOAR --input "{script}" --output "{audio}"`
	}
	replaced := strings.NewReplacer(
		"{script}", scriptPath,
		"{audio}", audioPath,
		"{voice}", "SOAR",
	).Replace(template)
	args, err := splitArgs(replaced)
	if err != nil {
		return "", nil, fmt.Errorf("parse tts args: %w", err)
	}
	return command, args, nil
}

func splitArgs(input string) ([]string, error) {
	args := []string{}
	var current strings.Builder
	var quote rune
	escaped := false
	for _, r := range input {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
				continue
			}
			current.WriteRune(r)
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r == ' ' || r == '\t' || r == '\n' {
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteRune(r)
	}
	if escaped {
		current.WriteRune('\\')
	}
	if quote != 0 {
		return nil, errors.New("unterminated quote")
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args, nil
}
