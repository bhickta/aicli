package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bhickta/aicli/internal/config"
)

type Runner interface {
	CombinedOutput(ctx context.Context, command string, args ...string) ([]byte, error)
	Start(ctx context.Context, command string, args ...string) error
}

type Service struct {
	tools          config.ToolConfig
	runner         Runner
	now            func() time.Time
	sendFocusDelay time.Duration
	sendRetryDelay time.Duration
}

func New(tools config.ToolConfig, runner Runner) *Service {
	return &Service{
		tools:          tools,
		runner:         runner,
		now:            time.Now,
		sendFocusDelay: 300 * time.Millisecond,
		sendRetryDelay: 1500 * time.Millisecond,
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
	attempts, err := s.activateAndSend(ctx, windowID, normalized.sendRetries)
	if err != nil {
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
		Output:         fmt.Sprintf("Enter keystroke delivered to active WhatsApp Web window %d time(s)", attempts),
		SendAttempts:   attempts,
	}, nil
}
