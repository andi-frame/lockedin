package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/andi-frame/lockedin/apps/server/internal/domain"
)

var ErrRateLimited = &domain.Error{Code: "auth.rate_limited", Msg: "too many attempts; wait a minute and try again"}

// Limiter is a fixed-window counter in Redis. Login uses it per IP and per email so
// neither spraying one account nor many accounts from one address works.
type Limiter struct {
	rdb    *redis.Client
	limit  int64
	window time.Duration
	prefix string
}

func NewLimiter(rdb *redis.Client, name string, limit int64, window time.Duration) *Limiter {
	return &Limiter{rdb: rdb, limit: limit, window: window, prefix: "rl:" + name + ":"}
}

// Allow counts one attempt for each key and fails if any key is over the limit.
func (l *Limiter) Allow(ctx context.Context, keys ...string) error {
	pipe := l.rdb.TxPipeline()
	incrs := make([]*redis.IntCmd, len(keys))
	for i, k := range keys {
		sum := sha256.Sum256([]byte(k))
		rk := l.prefix + hex.EncodeToString(sum[:8])
		incrs[i] = pipe.Incr(ctx, rk)
		pipe.ExpireNX(ctx, rk, l.window)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return err
	}
	for _, c := range incrs {
		if c.Val() > l.limit {
			return ErrRateLimited
		}
	}
	return nil
}
