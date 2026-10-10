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

// index is the set of a user's session keys. It exists so "end every other session" (a password
// change) does not have to scan Redis. It has no expiry of its own: a member whose session has
// expired is harmless and is dropped the next time the user logs out or the others are revoked.
func (s *Sessions) index(user uuid.UUID) string { return "usess:" + user.String() }

// Create returns a new random session token for the user.
func (s *Sessions) Create(ctx context.Context, user uuid.UUID) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	pipe := s.rdb.TxPipeline()
	pipe.Set(ctx, s.key(token), user.String(), SessionTTL)
	pipe.SAdd(ctx, s.index(user), s.key(token))
	if _, err := pipe.Exec(ctx); err != nil {
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

// Delete ends one session and takes it out of its user's index.
func (s *Sessions) Delete(ctx context.Context, token string) error {
	k := s.key(token)
	v, err := s.rdb.GetDel(ctx, k).Result()
	if errors.Is(err, redis.Nil) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("auth: delete session: %w", err)
	}
	if user, err := uuid.Parse(v); err == nil {
		return s.rdb.SRem(ctx, s.index(user), k).Err()
	}
	return nil
}

// RevokeOthers ends every session of the user except the one holding `keep` (which may be empty to
// end them all). It also drops index entries whose sessions have expired.
func (s *Sessions) RevokeOthers(ctx context.Context, user uuid.UUID, keep string) error {
	idx := s.index(user)
	members, err := s.rdb.SMembers(ctx, idx).Result()
	if err != nil {
		return fmt.Errorf("auth: list sessions: %w", err)
	}
	keepKey := ""
	if keep != "" {
		keepKey = s.key(keep)
	}
	var gone []any
	for _, k := range members {
		if k != keepKey {
			gone = append(gone, k)
		}
	}
	if len(gone) == 0 {
		return nil
	}
	pipe := s.rdb.TxPipeline()
	for _, k := range gone {
		pipe.Del(ctx, k.(string))
	}
	pipe.SRem(ctx, idx, gone...)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("auth: revoke sessions: %w", err)
	}
	return nil
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
