package storage

import (
	"context"
	"time"

	"github.com/bhickta/aicli/internal/execution"
)

func (s *SQLiteStore) LoadExecutionUsage(ctx context.Context, since time.Time) ([]execution.UsageEvent, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT profile_id, provider_id, model, occurred_at, reserved_tokens
FROM execution_usage_events
WHERE occurred_at >= ?
ORDER BY occurred_at`, since.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]execution.UsageEvent, 0)
	for rows.Next() {
		var event execution.UsageEvent
		if err := rows.Scan(
			&event.ProfileID,
			&event.ProviderID,
			&event.Model,
			&event.OccurredAt,
			&event.ReservedTokens,
		); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (s *SQLiteStore) RecordExecutionUsage(ctx context.Context, event execution.UsageEvent) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM execution_usage_events WHERE occurred_at < ?`, event.OccurredAt.UTC().Add(-48*time.Hour)); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `
INSERT INTO execution_usage_events(profile_id, provider_id, model, occurred_at, reserved_tokens)
VALUES(?, ?, ?, ?, ?)`,
		event.ProfileID,
		event.ProviderID,
		event.Model,
		event.OccurredAt.UTC(),
		event.ReservedTokens,
	); err != nil {
		return err
	}
	return nil
}
