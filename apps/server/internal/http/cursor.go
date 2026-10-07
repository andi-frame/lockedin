package http

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/andi-frame/lockedin/apps/server/internal/domain"
	"github.com/andi-frame/lockedin/apps/server/internal/http/api"
)

// Cursors are opaque to clients: base64url of a small JSON keyset. They only say where
// a page starts, and every query is already filtered to the caller's own data, so a
// forged cursor can move the window but never widen what is visible.

var errValidation = &domain.Error{Code: string(api.ValidationFailed), Msg: "the request is not valid"}

// invalid is a validation.failed error whose detail names the problem.
func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", errValidation, fmt.Sprintf(format, args...))
}

func encodeCursor(v any) *string {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	s := base64.RawURLEncoding.EncodeToString(raw)
	return &s
}

// decodeCursor fills v from an optional cursor. A nil or empty cursor means the first page.
func decodeCursor(c *string, v any) (bool, error) {
	if c == nil || strings.TrimSpace(*c) == "" {
		return false, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(*c)
	if err != nil || json.Unmarshal(raw, v) != nil {
		return false, invalid("cursor is not valid")
	}
	return true, nil
}

func limitOf(l *api.Limit) int {
	if l == nil {
		return 0 // the service applies its default
	}
	return *l
}

type ctxKey int

const clientIPKey ctxKey = iota

func withClientIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, clientIPKey, ip)
}

// ClientIPFromContext is the caller's address, for per-IP limits inside services.
func ClientIPFromContext(ctx context.Context) string {
	ip, _ := ctx.Value(clientIPKey).(string)
	return ip
}
