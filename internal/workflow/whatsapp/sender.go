package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

func (s *Service) findWhatsAppWindow(ctx context.Context) (string, error) {
	raw, err := s.runner.CombinedOutput(
		ctx,
		toolValue(s.tools.XDoTool, "xdotool"),
		"search",
		"--name",
		"WhatsApp",
	)
	if err != nil {
		return "", fmt.Errorf("find WhatsApp Firefox window: %w: %s", err, strings.TrimSpace(string(raw)))
	}

	fields := strings.Fields(string(raw))
	if len(fields) == 0 {
		return "", errors.New("WhatsApp Web window was not found")
	}

	windowID := fields[len(fields)-1]
	if _, err := strconv.ParseInt(windowID, 10, 64); err != nil {
		return "", fmt.Errorf("invalid xdotool window id %q", windowID)
	}
	return windowID, nil
}

func (s *Service) activateAndSend(ctx context.Context, windowID string, retries int) (int, error) {
	if retries <= 0 {
		retries = defaultSendRetries
	}
	if retries > maxSendRetries {
		retries = maxSendRetries
	}

	xdotool := toolValue(s.tools.XDoTool, "xdotool")
	if raw, err := s.runner.CombinedOutput(ctx, xdotool, "windowactivate", "--sync", windowID); err != nil {
		return 0, fmt.Errorf("activate WhatsApp window: %w: %s", err, strings.TrimSpace(string(raw)))
	}
	if s.sendFocusDelay > 0 {
		if err := sleepContext(ctx, s.sendFocusDelay); err != nil {
			return 0, err
		}
	}

	for attempt := 1; attempt <= retries; attempt++ {
		if raw, err := s.runner.CombinedOutput(ctx, xdotool, "key", "--clearmodifiers", "Return"); err != nil {
			return attempt - 1, fmt.Errorf("send WhatsApp message: %w: %s", err, strings.TrimSpace(string(raw)))
		}
		if attempt < retries && s.sendRetryDelay > 0 {
			if err := sleepContext(ctx, s.sendRetryDelay); err != nil {
				return attempt, err
			}
		}
	}
	return retries, nil
}
