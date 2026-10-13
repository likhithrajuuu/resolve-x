package authn

import (
	"context"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisRevoker invalidates every token issued to a user before a cut-off time.
// Entries expire with the longest possible token lifetime, so the set stays small.
type RedisRevoker struct {
	R   *redis.Client
	TTL time.Duration
}

func key(userID string) string { return "revoked_after:" + userID }

// RevokeUser makes all tokens issued up to now invalid.
func (r *RedisRevoker) RevokeUser(ctx context.Context, userID string) error {
	// +1 second: tokens carry whole-second issue times, so a token issued in the
	// same second as the revocation must also be rejected.
	return r.R.Set(ctx, key(userID), strconv.FormatInt(time.Now().Unix()+1, 10), r.TTL).Err()
}

// Revoked fails open on Redis errors: an outage must not lock every user out,
// and tokens still expire on their own within their TTL.
func (r *RedisRevoker) Revoked(ctx context.Context, c *Claims) bool {
	v, err := r.R.Get(ctx, key(c.Sub)).Result()
	if err != nil {
		return false
	}
	cut, err := strconv.ParseInt(v, 10, 64)
	return err == nil && c.Iat < cut
}
