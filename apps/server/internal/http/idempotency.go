package http

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/andi-frame/lockedin/apps/server/internal/auth"
	"github.com/andi-frame/lockedin/apps/server/internal/http/api"
)

const (
	idempotencyHeader = "Idempotency-Key"
	idempotencyTTL    = 24 * time.Hour
	// A request that dies before finishing releases its key after this long.
	idempotencyLockTTL = 30 * time.Second
	idempotencyMaxBody = 256 << 10
)

// record is what Redis keeps per key: first a "pending" marker, then the response.
type record struct {
	State       string `json:"state"` // pending | done
	Fingerprint string `json:"fp"`
	Status      int    `json:"status,omitempty"`
	ContentType string `json:"ct,omitempty"`
	Body        []byte `json:"body,omitempty"`
}

// idempotency replays the stored response when a client retries a mutating request
// with the same Idempotency-Key (ARCHITECTURE §3). The key is scoped by user, method
// and path, so one key can never replay another user's or another resource's answer.
//
// Only 2xx responses are stored. A failed attempt releases the key so the retry runs
// again, which is safe because every transition is transactional. Redis trouble
// fails open: the database's own idempotency keys and status guards still hold.
func idempotency(rdb *redis.Client, log *slog.Logger) fiber.Handler {
	return func(c fiber.Ctx) error {
		key := strings.TrimSpace(c.Get(idempotencyHeader))
		if key == "" || c.Method() == fiber.MethodGet || c.Method() == fiber.MethodHead || c.Method() == fiber.MethodOptions {
			return c.Next()
		}
		user := auth.UserID(c)
		if user == uuid.Nil {
			return c.Next()
		}
		if n := len(key); n < 8 || n > 128 {
			return problemError(api.ValidationFailed, "Idempotency-Key must be 8 to 128 characters")
		}

		ctx := c.Context()
		rkey := "idem:" + digest(user.String(), c.Method(), c.Path(), key)
		fp := digest(c.Method(), c.Path(), string(c.Request().URI().QueryString()), string(c.Body()))

		pending, _ := json.Marshal(record{State: "pending", Fingerprint: fp})
		won, err := rdb.SetNX(ctx, rkey, pending, idempotencyLockTTL).Result()
		if err != nil {
			log.Warn("idempotency store unavailable, continuing without it", "err", err)
			return c.Next()
		}
		if !won {
			return replay(c, rdb, rkey, fp)
		}

		finished := false
		defer func() {
			if !finished { // error, non-2xx, or panic: let the retry run again
				_ = rdb.Del(context.WithoutCancel(ctx), rkey).Err()
			}
		}()
		if err := c.Next(); err != nil {
			return err
		}
		status, body := c.Response().StatusCode(), c.Response().Body()
		if status < 200 || status > 299 || len(body) > idempotencyMaxBody {
			return nil
		}
		done, _ := json.Marshal(record{
			State: "done", Fingerprint: fp, Status: status,
			ContentType: string(c.Response().Header.ContentType()), Body: append([]byte(nil), body...),
		})
		if err := rdb.Set(context.WithoutCancel(ctx), rkey, done, idempotencyTTL).Err(); err != nil {
			log.Warn("could not store idempotent response", "err", err)
			return nil
		}
		finished = true
		return nil
	}
}

func replay(c fiber.Ctx, rdb *redis.Client, rkey, fp string) error {
	raw, err := rdb.Get(c.Context(), rkey).Bytes()
	var rec record
	if err != nil || json.Unmarshal(raw, &rec) != nil {
		// Released or expired between our SETNX and GET: treat as still running.
		c.Set(fiber.HeaderRetryAfter, "1")
		return problemError(api.IdempotencyInProgress, "this request is already being processed")
	}
	if rec.Fingerprint != fp {
		return problemError(api.IdempotencyKeyReused, "this Idempotency-Key was already used with a different request")
	}
	if rec.State != "done" {
		c.Set(fiber.HeaderRetryAfter, "1")
		return problemError(api.IdempotencyInProgress, "this request is already being processed")
	}
	c.Status(rec.Status)
	c.Set(fiber.HeaderContentType, rec.ContentType)
	c.Set("Idempotent-Replayed", "true")
	return c.Send(rec.Body)
}

func digest(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}
