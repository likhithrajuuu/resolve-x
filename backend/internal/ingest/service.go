// Package ingest implements the Ingestion Service: authenticate, rate limit,
// validate size, publish to telemetry.raw. Nothing expensive happens here.
package ingest

import (
	"context"
	"errors"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/resolvex/resolve-x/backend/internal/auth"
	"github.com/resolvex/resolve-x/backend/internal/telemetry"
)

var (
	ErrRateLimited = errors.New("tenant rate limit exceeded")
	ErrUnavailable = errors.New("backend unavailable")
)

type Service struct {
	Keys *auth.KeyStore
	Pub  *Publisher

	limiters sync.Map // tenantID -> *tenantLimiter
}

type tenantLimiter struct {
	rps int
	l   *rate.Limiter
}

// Accept authenticates, rate limits and publishes one export request.
// Errors: auth.ErrInvalidKey, ErrRateLimited, ErrUnavailable.
func (s *Service) Accept(ctx context.Context, rawKey string, sig telemetry.Signal, contentType, encoding string, body []byte) error {
	pr, err := s.Keys.Resolve(ctx, rawKey)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidKey) {
			return err
		}
		return ErrUnavailable
	}
	if !s.allow(pr) {
		return ErrRateLimited
	}
	start := time.Now()
	if err := s.Pub.Publish(ctx, pr, sig, contentType, encoding, body); err != nil {
		return ErrUnavailable
	}
	publishSeconds.Observe(time.Since(start).Seconds())
	payloadBytes.WithLabelValues(string(sig)).Observe(float64(len(body)))
	return nil
}

// allow is a per-instance token bucket. With N ingestion replicas a tenant's
// effective limit is N x rate; a global limiter (Redis) is a later step.
func (s *Service) allow(pr *auth.Principal) bool {
	if pr.RatePerSec <= 0 {
		return true
	}
	if v, ok := s.limiters.Load(pr.TenantID); ok {
		tl := v.(*tenantLimiter)
		if tl.rps == pr.RatePerSec {
			return tl.l.Allow()
		}
	}
	tl := &tenantLimiter{rps: pr.RatePerSec, l: rate.NewLimiter(rate.Limit(pr.RatePerSec), pr.RatePerSec*2)}
	s.limiters.Store(pr.TenantID, tl)
	return tl.l.Allow()
}
