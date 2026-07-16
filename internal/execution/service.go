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
}

type profileRuntime struct {
	config    config.ExecutionProfile
	semaphore chan struct{}
}

func New(profiles []config.ExecutionProfile, providerFor ProviderFor) *Service {
	service := &Service{providerFor: providerFor, cooldowns: make(map[string]time.Time)}
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
	for _, target := range profile.Targets {
		if !target.Enabled || s.inCooldown(profile.ID, target) {
			continue
		}
		candidate, ok := s.providerFor(target.ProviderID)
		if !ok {
			response.Attempts = append(response.Attempts, failedAttempt(target, "provider not found"))
			continue
		}
		result, err := executeTarget(ctx, candidate, profile.Capability, target.Model, request)
		if err == nil {
			result.CorrelationID, result.Profile = request.CorrelationID, profile.ID
			result.Capability, result.ProviderID, result.Model = profile.Capability, target.ProviderID, target.Model
			result.Attempts = append(response.Attempts, Attempt{ProviderID: target.ProviderID, Model: target.Model, Status: "success"})
			result.EstimatedCost = estimateCost(result.Usage, target)
			return result, nil
		}
		response.Attempts = append(response.Attempts, failedAttempt(target, err.Error()))
		if isRateLimit(err) {
			s.setCooldown(profile, target)
		}
	}
	if len(response.Attempts) == 0 {
		return response, ErrNoTargets
	}
	return response, fmt.Errorf("all execution targets failed: %s", response.Attempts[len(response.Attempts)-1].Error)
}

func (s *Service) inCooldown(profileID string, target config.ExecutionTarget) bool {
	s.mu.RLock()
	until := s.cooldowns[targetKey(profileID, target)]
	s.mu.RUnlock()
	return time.Now().Before(until)
}

func (s *Service) setCooldown(profile config.ExecutionProfile, target config.ExecutionTarget) {
	s.mu.Lock()
	s.cooldowns[targetKey(profile.ID, target)] = time.Now().Add(time.Duration(profile.CooldownSeconds) * time.Second)
	s.mu.Unlock()
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

func targetKey(profileID string, target config.ExecutionTarget) string {
	return profileID + "\x00" + target.ProviderID + "\x00" + target.Model
}

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
