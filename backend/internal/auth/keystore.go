// Package auth resolves API keys to tenant context.
//
// Lookups go in-process cache -> Redis -> PostgreSQL so the request path
// does not hit the database for every export (ingestion-service.md, Data).
package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

var ErrInvalidKey = errors.New("invalid or revoked api key")

// Principal is the tenant context derived from an API key.
type Principal struct {
	TenantID      string `json:"tenantId"`
	ProjectID     string `json:"projectId"`
	EnvironmentID string `json:"environmentId"`
	RatePerSec    int    `json:"ratePerSec"`
}

func HashKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func CacheKey(hash string) string { return "apikey:" + hash }

type localEntry struct {
	p       *Principal // nil = known-invalid
	expires time.Time
}

type KeyStore struct {
	db       *pgxpool.Pool
	rdb      *redis.Client
	local    sync.Map // hash -> localEntry
	sf       singleflight.Group
	LocalTTL time.Duration
	RedisTTL time.Duration
	NegTTL   time.Duration
}

func NewKeyStore(db *pgxpool.Pool, rdb *redis.Client) *KeyStore {
	return &KeyStore{db: db, rdb: rdb, LocalTTL: 15 * time.Second, RedisTTL: 5 * time.Minute, NegTTL: 10 * time.Second}
}

// Resolve returns the principal for a raw API key or ErrInvalidKey.
// Any other error means the backing stores are unavailable (retryable).
func (k *KeyStore) Resolve(ctx context.Context, raw string) (*Principal, error) {
	hash := HashKey(raw)
	if v, ok := k.local.Load(hash); ok {
		e := v.(localEntry)
		if time.Now().Before(e.expires) {
			if e.p == nil {
				return nil, ErrInvalidKey
			}
			return e.p, nil
		}
	}
	v, err, _ := k.sf.Do(hash, func() (any, error) { return k.load(ctx, hash) })
	if err != nil {
		return nil, err
	}
	p, _ := v.(*Principal)
	if p == nil {
		return nil, ErrInvalidKey
	}
	return p, nil
}

func (k *KeyStore) load(ctx context.Context, hash string) (*Principal, error) {
	if k.rdb != nil {
		if b, err := k.rdb.Get(ctx, CacheKey(hash)).Bytes(); err == nil {
			if string(b) == "null" {
				k.remember(hash, nil)
				return nil, nil
			}
			var p Principal
			if json.Unmarshal(b, &p) == nil {
				k.remember(hash, &p)
				return &p, nil
			}
		}
		// Redis errors fall through to PostgreSQL; the cache is an optimisation.
	}
	var p Principal
	err := k.db.QueryRow(ctx, `
		SELECT k.tenant_id, k.project_id, k.environment_id, t.rate_per_sec
		FROM api_keys k JOIN tenants t ON t.id = k.tenant_id
		WHERE k.key_hash = $1 AND k.status = 'active'`, hash).
		Scan(&p.TenantID, &p.ProjectID, &p.EnvironmentID, &p.RatePerSec)
	if errors.Is(err, pgx.ErrNoRows) {
		k.store(ctx, hash, nil, k.NegTTL)
		k.remember(hash, nil)
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	k.store(ctx, hash, &p, k.RedisTTL)
	k.remember(hash, &p)
	return &p, nil
}

func (k *KeyStore) remember(hash string, p *Principal) {
	ttl := k.LocalTTL
	if p == nil {
		ttl = k.NegTTL
	}
	k.local.Store(hash, localEntry{p: p, expires: time.Now().Add(ttl)})
}

func (k *KeyStore) store(ctx context.Context, hash string, p *Principal, ttl time.Duration) {
	if k.rdb == nil {
		return
	}
	b := []byte("null")
	if p != nil {
		b, _ = json.Marshal(p)
	}
	_ = k.rdb.Set(ctx, CacheKey(hash), b, ttl).Err()
}
