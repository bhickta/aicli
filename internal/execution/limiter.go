package execution

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bhickta/aicli/internal/config"
)

const (
	usageRetention       = 25 * time.Hour
	minuteWindow         = 60 * time.Second
	minuteWindowHeadroom = time.Second
	charsPerToken        = 3
	minimumOutputReserve = 4096
)

type UsageEvent struct {
	ProfileID      string
	ProviderID     string
	Model          string
	OccurredAt     time.Time
	ReservedTokens int
}

type UsageStore interface {
	LoadExecutionUsage(context.Context, time.Time) ([]UsageEvent, error)
	RecordExecutionUsage(context.Context, UsageEvent) error
}

type targetUsage struct {
	events []UsageEvent
	active int
}

type reservationResult struct {
	ready          bool
	rateLimited    bool
	dailyExhausted bool
	activeBlocked  bool
	wait           time.Duration
}

func (s *Service) loadUsage() {
	if s.usageStore == nil {
		return
	}
	events, err := s.usageStore.LoadExecutionUsage(context.Background(), s.now().Add(-usageRetention))
	if err != nil {
		s.usageErr = fmt.Errorf("load persisted execution usage: %w", err)
		return
	}
	for _, event := range events {
		key := usageKey(config.ExecutionTarget{ProviderID: event.ProviderID, Model: event.Model})
		state := s.usage[key]
		if state == nil {
			state = &targetUsage{}
			s.usage[key] = state
		}
		state.events = append(state.events, event)
	}
}

func (s *Service) targetsFor(profile config.ExecutionProfile) []config.ExecutionTarget {
	targets := append([]config.ExecutionTarget(nil), profile.Targets...)
	if profile.SelectionStrategy != config.SelectionRoundRobin || len(targets) < 2 {
		return targets
	}

	s.mu.Lock()
	offset := s.rotation[profile.ID]
	s.rotation[profile.ID] = offset + 1
	s.mu.Unlock()

	for start := 0; start < len(targets); {
		end := start + 1
		for end < len(targets) && targets[end].Priority == targets[start].Priority {
			end++
		}
		group := append([]config.ExecutionTarget(nil), targets[start:end]...)
		shift := offset % len(group)
		copy(targets[start:end], append(group[shift:], group[:shift]...))
		start = end
	}
	return targets
}

func (s *Service) reserve(
	ctx context.Context,
	profile config.ExecutionProfile,
	target config.ExecutionTarget,
	reservedTokens int,
) (reservationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.usageErr != nil && hasRateLimit(target.RateLimit) {
		return reservationResult{}, s.usageErr
	}
	now := s.now().UTC()
	if until := s.cooldowns[usageKey(target)]; now.Before(until) {
		return reservationResult{rateLimited: true, wait: until.Sub(now)}, nil
	}

	key := usageKey(target)
	state := s.usage[key]
	if state == nil {
		state = &targetUsage{}
		s.usage[key] = state
	}
	state.events = pruneUsage(state.events, now.Add(-usageRetention))
	if target.MaxConcurrency > 0 && state.active >= target.MaxConcurrency {
		return reservationResult{activeBlocked: true}, nil
	}

	availability := evaluateRateLimit(target.RateLimit, state.events, reservedTokens, now)
	if !availability.ready {
		return availability, nil
	}
	event := UsageEvent{
		ProfileID:      profile.ID,
		ProviderID:     target.ProviderID,
		Model:          target.Model,
		OccurredAt:     now,
		ReservedTokens: reservedTokens,
	}
	if s.usageStore != nil && hasRateLimit(target.RateLimit) {
		if err := s.usageStore.RecordExecutionUsage(ctx, event); err != nil {
			return reservationResult{}, fmt.Errorf("persist execution usage reservation: %w", err)
		}
	}
	if hasRateLimit(target.RateLimit) {
		state.events = append(state.events, event)
	}
	state.active++
	return reservationResult{ready: true}, nil
}

func (s *Service) release(target config.ExecutionTarget) {
	s.mu.Lock()
	state := s.usage[usageKey(target)]
	if state != nil && state.active > 0 {
		state.active--
	}
	s.mu.Unlock()
	s.notify()
}

func (s *Service) waitUntilAvailable(ctx context.Context, wait time.Duration, activeBlocked bool) error {
	if !activeBlocked && wait <= 0 {
		return ErrNoTargets
	}
	if wait == time.Duration(1<<63-1) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.wake:
			return nil
		}
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.wake:
		return nil
	case <-timer.C:
		return nil
	}
}

func (s *Service) notify() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func evaluateRateLimit(
	limit config.TargetRateLimit,
	events []UsageEvent,
	reservedTokens int,
	now time.Time,
) reservationResult {
	if !hasRateLimit(limit) {
		return reservationResult{ready: true}
	}
	today := now.Format("2006-01-02")
	minuteStart := now.Add(-minuteWindow)
	minuteEvents := make([]UsageEvent, 0, len(events))
	dayRequests := 0
	minuteTokens := 0
	for _, event := range events {
		if event.OccurredAt.UTC().Format("2006-01-02") == today {
			dayRequests++
		}
		if event.OccurredAt.After(minuteStart) {
			minuteEvents = append(minuteEvents, event)
			minuteTokens += event.ReservedTokens
		}
	}
	if limit.RequestsPerDay > 0 && dayRequests >= limit.RequestsPerDay {
		return reservationResult{rateLimited: true, dailyExhausted: true}
	}

	wait := time.Duration(0)
	if limit.RequestsPerMinute > 0 && len(minuteEvents) >= limit.RequestsPerMinute {
		sort.Slice(minuteEvents, func(i, j int) bool { return minuteEvents[i].OccurredAt.Before(minuteEvents[j].OccurredAt) })
		wait = minuteEvents[0].OccurredAt.Add(minuteWindow + minuteWindowHeadroom).Sub(now)
	}
	if limit.TokensPerMinute > 0 && minuteTokens+reservedTokens > limit.TokensPerMinute {
		tokenWait := tokenAvailabilityWait(minuteEvents, minuteTokens, reservedTokens, limit.TokensPerMinute, now)
		if tokenWait > wait {
			wait = tokenWait
		}
	}
	if wait > 0 {
		return reservationResult{rateLimited: true, wait: wait}
	}
	return reservationResult{ready: true}
}

func tokenAvailabilityWait(
	events []UsageEvent,
	currentTokens, reservedTokens, tokenLimit int,
	now time.Time,
) time.Duration {
	ordered := append([]UsageEvent(nil), events...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].OccurredAt.Before(ordered[j].OccurredAt) })
	remaining := currentTokens
	for _, event := range ordered {
		remaining -= event.ReservedTokens
		if remaining+reservedTokens <= tokenLimit {
			return event.OccurredAt.Add(minuteWindow + minuteWindowHeadroom).Sub(now)
		}
	}
	return minuteWindow + minuteWindowHeadroom
}

func estimateRequestTokens(request Request) int {
	characters := len(request.Prompt) + len(request.Query)
	for _, message := range request.Messages {
		characters += len(message.Role) + len(message.Content)
	}
	for _, input := range request.Inputs {
		characters += len(input)
	}
	for _, document := range request.Documents {
		characters += len(document)
	}
	inputTokens := max(1, (characters+charsPerToken-1)/charsPerToken)
	outputReserve := request.MaxTokens
	if outputReserve < minimumOutputReserve {
		outputReserve = max(minimumOutputReserve, inputTokens*15/100)
	}
	return inputTokens + outputReserve
}

func hasRateLimit(limit config.TargetRateLimit) bool {
	return limit.RequestsPerMinute > 0 || limit.TokensPerMinute > 0 || limit.RequestsPerDay > 0
}

func pruneUsage(events []UsageEvent, cutoff time.Time) []UsageEvent {
	kept := events[:0]
	for _, event := range events {
		if event.OccurredAt.After(cutoff) {
			kept = append(kept, event)
		}
	}
	return kept
}

func usageKey(target config.ExecutionTarget) string {
	return strings.TrimSpace(target.ProviderID) + "\x00" + strings.TrimSpace(target.Model)
}
