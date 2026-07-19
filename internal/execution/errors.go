package execution

import "errors"

var (
	ErrDisabled         = errors.New("execution profile is disabled")
	ErrProfileNotFound  = errors.New("execution profile was not found")
	ErrCapability       = errors.New("requested capability does not match the execution profile")
	ErrNoTargets        = errors.New("execution profile has no available targets")
	ErrRateLimited      = errors.New("execution profile is waiting for configured rate limits")
	ErrDailyRateLimit   = errors.New("daily request limit reached for every available execution target")
	ErrEmbeddingSupport = errors.New("provider does not support embeddings")
	ErrRerankingSupport = errors.New("provider does not support reranking")
)
