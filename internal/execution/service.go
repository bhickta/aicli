package execution

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
)

type ProviderFor func(string) (provider.Provider, bool)

type Service struct {
	providerFor ProviderFor
	mu          sync.RWMutex
	profiles    map[string]*profileRuntime
	cooldowns   map[string]time.Time
	usage       map[string]*targetUsage
	rotation    map[string]int
	usageStore  UsageStore
	usageErr    error
	wake        chan struct{}
	now         func() time.Time
}

type profileRuntime struct {
	config    config.ExecutionProfile
	semaphore chan struct{}
}

func New(profiles []config.ExecutionProfile, providerFor ProviderFor) *Service {
	return NewWithUsageStore(profiles, providerFor, nil)
}

func NewWithUsageStore(profiles []config.ExecutionProfile, providerFor ProviderFor, usageStore UsageStore) *Service {
	service := &Service{
		providerFor: providerFor,
		cooldowns:   make(map[string]time.Time),
		usage:       make(map[string]*targetUsage),
		rotation:    make(map[string]int),
		usageStore:  usageStore,
		wake:        make(chan struct{}, 1),
		now:         time.Now,
	}
	service.loadUsage()
	service.UpdateProfiles(profiles)
	return service
}

func (s *Service) UpdateProfiles(profiles []config.ExecutionProfile) {
	runtimes := make(map[string]*profileRuntime, len(profiles))
	for _, profile := range config.NormalizeExecutionProfiles(profiles) {
		runtimes[profile.ID] = &profileRuntime{
			config: profile, semaphore: make(chan struct{}, profile.MaxConcurrency),
		}
	}
	s.mu.Lock()
	s.profiles = runtimes
	s.mu.Unlock()
}

func (s *Service) Execute(ctx context.Context, request Request) (Response, error) {
	started := time.Now()
	runtime, err := s.runtimeFor(request)
	if err != nil {
		return Response{}, err
	}
	if err := acquire(ctx, runtime.semaphore); err != nil {
		return Response{}, err
	}
	defer release(runtime.semaphore)
	ctx, cancel := context.WithTimeout(ctx, time.Duration(runtime.config.TimeoutSeconds)*time.Second)
	defer cancel()
	response, err := s.tryTargets(ctx, runtime.config, request)
	response.DurationMS = time.Since(started).Milliseconds()
	return response, err
}

func (s *Service) runtimeFor(request Request) (*profileRuntime, error) {
	s.mu.RLock()
	runtime := s.profiles[request.Profile]
	s.mu.RUnlock()
	if runtime == nil {
		return nil, ErrProfileNotFound
	}
	if !runtime.config.Enabled {
		return nil, ErrDisabled
	}
	if request.Capability != "" && request.Capability != runtime.config.Capability {
		return nil, ErrCapability
	}
	return runtime, nil
}

func (s *Service) tryTargets(ctx context.Context, profile config.ExecutionProfile, request Request) (Response, error) {
	response := Response{CorrelationID: request.CorrelationID, Profile: profile.ID, Capability: profile.Capability}
	targets := s.targetsFor(profile)
	reservedTokens := estimateRequestTokens(request)
	for {
		shortestWait := time.Duration(1<<63 - 1)
		activeBlocked := false
		dailyExhausted, limitedTargets := 0, 0

		for _, target := range targets {
			if !target.Enabled {
				continue
			}
			candidate, ok := s.providerFor(target.ProviderID)
			if !ok {
				response.Attempts = append(response.Attempts, failedAttempt(target, "provider not found"))
				continue
			}
			reservation, err := s.reserve(ctx, profile, target, reservedTokens)
			if err != nil {
				return response, err
			}
			if !reservation.ready {
				if reservation.rateLimited {
					limitedTargets++
				}
				if reservation.dailyExhausted {
					dailyExhausted++
				}
				if reservation.activeBlocked {
					activeBlocked = true
				}
				if reservation.wait > 0 && reservation.wait < shortestWait {
					shortestWait = reservation.wait
				}
				continue
			}

			result, targetErr := executeTarget(ctx, candidate, profile.Capability, target.Model, request)
			s.release(target)
			if targetErr == nil {
				result.CorrelationID, result.Profile = request.CorrelationID, profile.ID
				result.Capability, result.ProviderID, result.Model = profile.Capability, target.ProviderID, target.Model
				result.Attempts = append(response.Attempts, Attempt{ProviderID: target.ProviderID, Model: target.Model, Status: "success"})
				result.EstimatedCost = estimateCost(result.Usage, target)
				return result, nil
			}
			response.Attempts = append(response.Attempts, failedAttempt(target, targetErr.Error()))
			if isRateLimit(targetErr) {
				s.setCooldown(profile, target)
			}
		}

		if len(response.Attempts) > 0 {
			return response, fmt.Errorf("all execution targets failed: %s", response.Attempts[len(response.Attempts)-1].Error)
		}
		if limitedTargets > 0 && dailyExhausted == limitedTargets {
			return response, ErrDailyRateLimit
		}
		if !activeBlocked && shortestWait == time.Duration(1<<63-1) {
			return response, ErrNoTargets
		}
		if deadline, ok := ctx.Deadline(); ok && shortestWait != time.Duration(1<<63-1) && s.now().Add(shortestWait).After(deadline) {
			return response, fmt.Errorf("%w: retry after %s", ErrRateLimited, shortestWait.Round(time.Second))
		}
		if err := s.waitUntilAvailable(ctx, shortestWait, activeBlocked); err != nil {
			return response, err
		}
	}
}

func (s *Service) setCooldown(profile config.ExecutionProfile, target config.ExecutionTarget) {
	s.mu.Lock()
	s.cooldowns[usageKey(target)] = s.now().Add(time.Duration(profile.CooldownSeconds) * time.Second)
	s.mu.Unlock()
	s.notify()
}

func acquire(ctx context.Context, semaphore chan struct{}) error {
	select {
	case semaphore <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func release(semaphore chan struct{}) { <-semaphore }

func failedAttempt(target config.ExecutionTarget, message string) Attempt {
	return Attempt{ProviderID: target.ProviderID, Model: target.Model, Status: "error", Error: message}
}

func isRateLimit(err error) bool {
	message := strings.ToLower(err.Error())
	return errors.Is(err, context.DeadlineExceeded) || strings.Contains(message, "429") ||
		strings.Contains(message, "rate limit") || strings.Contains(message, "quota")
}

func estimateCost(usage *provider.TokenUsage, target config.ExecutionTarget) float64 {
	if usage == nil {
		return 0
	}
	input := float64(usage.InputTokens) * target.InputCostPerMillion / 1_000_000
	output := float64(usage.OutputTokens) * target.OutputCostPerMillion / 1_000_000
	return input + output
}
