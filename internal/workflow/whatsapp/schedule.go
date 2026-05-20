package whatsapp

import (
	"context"
	"errors"
	"strings"
	"time"

	progressmodel "github.com/bhickta/aicli/internal/progress"
)

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
	return time.Time{}, errors.New("scheduled_at must be RFC3339 or YYYY-MM-DDTHH:MM in IST")
}

func istLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err == nil {
		return loc
	}
	return time.FixedZone("IST", 5*60*60+30*60)
}

func waitUntil(ctx context.Context, scheduledAt time.Time, startedAt time.Time, now func() time.Time, progress ProgressFunc) error {
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
			progress(progressmodel.Timed("waiting until "+scheduledAt.Format(time.RFC3339), startedAt, scheduledAt))
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
