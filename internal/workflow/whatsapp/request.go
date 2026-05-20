package whatsapp

import (
	"errors"
	"strings"
	"time"
	"unicode"
)

const (
	defaultWaitSeconds = 12
	minWaitSeconds     = 3
	maxWaitSeconds     = 120
	defaultSendRetries = 2
	minSendRetries     = 1
	maxSendRetries     = 5
)

type normalizedRequest struct {
	recipientName  string
	recipientPhone string
	message        string
	scheduledAt    time.Time
	autoSend       bool
	waitSeconds    int
	sendRetries    int
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

	scheduledAt, err := parseScheduledAt(req.ScheduledAt, istLocation())
	if err != nil {
		return normalizedRequest{}, err
	}
	if scheduledAt.Before(now.Add(-30 * time.Second)) {
		return normalizedRequest{}, errors.New("scheduled_at is in the past")
	}

	return normalizedRequest{
		recipientName:  recipientName,
		recipientPhone: recipientPhone,
		message:        message,
		scheduledAt:    scheduledAt,
		autoSend:       req.AutoSend,
		waitSeconds:    boundedWaitSeconds(req.WaitSeconds),
		sendRetries:    boundedSendRetries(req.SendRetries),
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

func boundedWaitSeconds(value int) int {
	if value <= 0 {
		value = defaultWaitSeconds
	}
	if value < minWaitSeconds {
		return minWaitSeconds
	}
	if value > maxWaitSeconds {
		return maxWaitSeconds
	}
	return value
}

func boundedSendRetries(value int) int {
	if value <= 0 {
		value = defaultSendRetries
	}
	if value < minSendRetries {
		return minSendRetries
	}
	if value > maxSendRetries {
		return maxSendRetries
	}
	return value
}
