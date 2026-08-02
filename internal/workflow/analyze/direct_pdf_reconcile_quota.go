package analyze

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/bhickta/aicli/internal/provider"
)

const directPDFReconciliationQuotaWindow = 61 * time.Second

type directPDFReconciliationQuotaGate struct {
	mu          sync.Mutex
	lastStarted time.Time
}

var directPDFReconciliationQuotaGates sync.Map

func (s *Service) runDirectPDFReconciliationDocument(
	ctx context.Context,
	processor provider.DocumentProcessor,
	request provider.DocumentRequest,
) (provider.DocumentResponse, error) {
	providerID := strings.ToLower(strings.TrimSpace(providerID(s.ocrProvider)))
	if !strings.HasPrefix(providerID, "gemini-lane-") {
		return processor.Document(ctx, request)
	}
	key := providerID + "\x00" + strings.ToLower(strings.TrimSpace(request.Model))
	value, _ := directPDFReconciliationQuotaGates.LoadOrStore(key, &directPDFReconciliationQuotaGate{})
	gate := value.(*directPDFReconciliationQuotaGate)
	gate.mu.Lock()
	defer gate.mu.Unlock()

	if delay := directPDFReconciliationQuotaDelay(gate.lastStarted, time.Now()); delay > 0 {
		s.logInfo(
			"waiting for Gemini reconciliation quota window",
			"provider", providerID,
			"model", request.Model,
			"delay_ms", delay.Milliseconds(),
		)
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return provider.DocumentResponse{}, ctx.Err()
		case <-timer.C:
		}
	}
	gate.lastStarted = time.Now()
	return processor.Document(ctx, request)
}

func directPDFReconciliationQuotaDelay(lastStarted time.Time, now time.Time) time.Duration {
	if lastStarted.IsZero() {
		return 0
	}
	remaining := lastStarted.Add(directPDFReconciliationQuotaWindow).Sub(now)
	if remaining <= 0 {
		return 0
	}
	return remaining
}
