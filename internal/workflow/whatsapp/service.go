package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/bhickta/aicli/internal/config"
)

const (
	defaultWaitSeconds = 12
	minWaitSeconds     = 3
	maxWaitSeconds     = 120
)

type Runner interface {
	CombinedOutput(ctx context.Context, command string, args ...string) ([]byte, error)
	Start(ctx context.Context, command string, args ...string) error
}

type Service struct {
	tools  config.ToolConfig
	runner Runner
	now    func() time.Time
}

func New(tools config.ToolConfig, runner Runner) *Service {
	return &Service{
		tools:  tools,
		runner: runner,
		now:    time.Now,
	}
}

func (s *Service) Schedule(ctx context.Context, req ScheduleRequest, progress ProgressFunc) (ScheduleResponse, error) {
	normalized, err := normalizeRequest(req, s.now())
	if err != nil {
		return ScheduleResponse{}, err
	}
	if s.runner == nil {
		return ScheduleResponse{}, errors.New("runner is required")
	}

	if progress != nil {
		progress("waiting until scheduled time", 1, 5)
	}
	if err := waitUntil(ctx, normalized.scheduledAt, s.now, progress); err != nil {
		return ScheduleResponse{}, err
	}

	chatURL := whatsappURL(normalized.recipientPhone, normalized.message)
	if progress != nil {
		progress("opening WhatsApp Web in Firefox", 2, 5)
	}
	if err := s.runner.Start(ctx, toolValue(s.tools.Firefox, "firefox"), chatURL); err != nil {
		return ScheduleResponse{}, fmt.Errorf("open firefox: %w", err)
	}
	if !normalized.autoSend {
		if progress != nil {
			progress("draft opened in WhatsApp Web", 5, 5)
		}
		return ScheduleResponse{
			RecipientName:  normalized.recipientName,
			RecipientPhone: normalized.recipientPhone,
			ScheduledAt:    normalized.scheduledAt.Format(time.RFC3339),
			AutoSend:       false,
			URL:            chatURL,
			Output:         "draft opened; auto-send was disabled",
		}, nil
	}

	if progress != nil {
		progress("waiting for WhatsApp Web message box", 3, 5)
	}
	if err := sleepContext(ctx, time.Duration(normalized.waitSeconds)*time.Second); err != nil {
		return ScheduleResponse{}, err
	}
	if progress != nil {
		progress("sending WhatsApp message", 4, 5)
	}
	windowID, err := s.findWhatsAppWindow(ctx)
	if err != nil {
		return ScheduleResponse{}, err
	}
	if err := s.activateAndSend(ctx, windowID); err != nil {
		return ScheduleResponse{}, err
	}
	if progress != nil {
		progress("completed", 5, 5)
	}
	return ScheduleResponse{
		RecipientName:  normalized.recipientName,
		RecipientPhone: normalized.recipientPhone,
		ScheduledAt:    normalized.scheduledAt.Format(time.RFC3339),
		AutoSend:       true,
		URL:            chatURL,
		Output:         "send keystroke delivered to WhatsApp Web",
	}, nil
}

type normalizedRequest struct {
	recipientName  string
	recipientPhone string
	message        string
	scheduledAt    time.Time
	autoSend       bool
	waitSeconds    int
}

func normalizeRequest(req ScheduleRequest, now time.Time) (normalizedRequest, error) {
	phoneInput := firstNonBlank(req.RecipientPhone, req.Recipient)
	recipientPhone, err := normalizeRecipient(phoneInput)
	if err != nil {
		return normalizedRequest{}, err
	}
	recipientName := strings.TrimSpace(req.RecipientName)
	if recipientName == "" && strings.TrimSpace(req.Recipient) != phoneInput {
		recipientName = strings.TrimSpace(req.Recipient)
	}
	message := strings.TrimSpace(req.Message)
	if message == "" {
		return normalizedRequest{}, errors.New("message is required")
	}
	scheduledAt, err := parseScheduledAt(req.ScheduledAt, now.Location())
	if err != nil {
		return normalizedRequest{}, err
	}
	if scheduledAt.Before(now.Add(-30 * time.Second)) {
		return normalizedRequest{}, errors.New("scheduled_at is in the past")
	}
	waitSeconds := req.WaitSeconds
	if waitSeconds <= 0 {
		waitSeconds = defaultWaitSeconds
	}
	if waitSeconds < minWaitSeconds {
		waitSeconds = minWaitSeconds
	}
	if waitSeconds > maxWaitSeconds {
		waitSeconds = maxWaitSeconds
	}
	return normalizedRequest{
		recipientName:  recipientName,
		recipientPhone: recipientPhone,
		message:        message,
		scheduledAt:    scheduledAt,
		autoSend:       req.AutoSend,
		waitSeconds:    waitSeconds,
	}, nil
}

func normalizeRecipient(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("recipient phone number is required")
	}
	var digits strings.Builder
	for _, r := range value {
		switch {
		case unicode.IsDigit(r):
			digits.WriteRune(r)
		case r == '+' || r == ' ' || r == '-' || r == '(' || r == ')':
		default:
			return "", errors.New("recipient must be a phone number with country code")
		}
	}
	out := digits.String()
	if len(out) < 8 || len(out) > 15 {
		return "", errors.New("recipient phone number must include country code and contain 8 to 15 digits")
	}
	return out, nil
}

func parseScheduledAt(value string, loc *time.Location) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, errors.New("scheduled_at is required")
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if layout == time.RFC3339 {
			if parsed, err := time.Parse(layout, value); err == nil {
				return parsed, nil
			}
			continue
		}
		if parsed, err := time.ParseInLocation(layout, value, loc); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, errors.New("scheduled_at must be RFC3339 or YYYY-MM-DDTHH:MM")
}

func waitUntil(ctx context.Context, scheduledAt time.Time, now func() time.Time, progress ProgressFunc) error {
	for {
		remaining := time.Until(scheduledAt)
		if now != nil {
			remaining = scheduledAt.Sub(now())
		}
		if remaining <= 0 {
			return nil
		}
		wait := remaining
		if wait > time.Minute {
			wait = time.Minute
		}
		if progress != nil {
			progress("waiting until "+scheduledAt.Format(time.RFC3339), 1, 5)
		}
		if err := sleepContext(ctx, wait); err != nil {
			return err
		}
	}
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func whatsappURL(recipient string, message string) string {
	query := url.Values{}
	query.Set("phone", recipient)
	query.Set("text", message)
	return "https://web.whatsapp.com/send?" + query.Encode()
}

func (s *Service) findWhatsAppWindow(ctx context.Context) (string, error) {
	raw, err := s.runner.CombinedOutput(ctx, toolValue(s.tools.XDoTool, "xdotool"), "search", "--name", "WhatsApp")
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

func (s *Service) activateAndSend(ctx context.Context, windowID string) error {
	xdotool := toolValue(s.tools.XDoTool, "xdotool")
	if raw, err := s.runner.CombinedOutput(ctx, xdotool, "windowactivate", "--sync", windowID); err != nil {
		return fmt.Errorf("activate WhatsApp window: %w: %s", err, strings.TrimSpace(string(raw)))
	}
	if raw, err := s.runner.CombinedOutput(ctx, xdotool, "key", "--window", windowID, "Return"); err != nil {
		return fmt.Errorf("send WhatsApp message: %w: %s", err, strings.TrimSpace(string(raw)))
	}
	return nil
}

func toolValue(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
