package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	SessionCookie = "tepati_session"
	CSRFCookie    = "tepati_csrf"
	CSRFHeader    = "X-CSRF-Token"
	SessionTTL    = 30 * 24 * time.Hour // sliding
)

var ErrNoSession = errors.New("auth: no valid session")

// Sessions stores opaque session tokens in Redis. Only sha256(token) is used as the
// key, so a Redis dump does not contain usable tokens.
type Sessions struct {
	rdb    *redis.Client
	secret []byte
	prefix string
}

func NewSessions(rdb *redis.Client, sessionSecret string) *Sessions {
	return &Sessions{rdb: rdb, secret: []byte(sessionSecret), prefix: "sess:"}
}

func (s *Sessions) key(token string) string {
	sum := sha256.Sum256([]byte(token))
	return s.prefix + hex.EncodeToString(sum[:])
}

// Create returns a new random session token for the user.
func (s *Sessions) Create(ctx context.Context, user uuid.UUID) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	if err := s.rdb.Set(ctx, s.key(token), user.String(), SessionTTL).Err(); err != nil {
		return "", fmt.Errorf("auth: store session: %w", err)
	}
	return token, nil
}

// Lookup resolves a token to its user and slides the expiry forward.
func (s *Sessions) Lookup(ctx context.Context, token string) (uuid.UUID, error) {
	if token == "" {
		return uuid.Nil, ErrNoSession
	}
	k := s.key(token)
	v, err := s.rdb.GetEx(ctx, k, SessionTTL).Result()
	if errors.Is(err, redis.Nil) {
		return uuid.Nil, ErrNoSession
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("auth: lookup session: %w", err)
	}
	id, err := uuid.Parse(v)
	if err != nil {
		return uuid.Nil, ErrNoSession
	}
	return id, nil
}

func (s *Sessions) Delete(ctx context.Context, token string) error {
	return s.rdb.Del(ctx, s.key(token)).Err()
}

// CSRFToken is HMAC(secret, session token): a double-submit token bound to the
// session, so an attacker cannot plant a matching cookie/header pair.
func (s *Sessions) CSRFToken(sessionToken string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte("csrf:" + sessionToken))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *Sessions) ValidCSRF(sessionToken, header string) bool {
	if sessionToken == "" || header == "" {
		return false
	}
	return hmac.Equal([]byte(s.CSRFToken(sessionToken)), []byte(header))
}
