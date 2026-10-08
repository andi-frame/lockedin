//go:build integration

package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/andi-frame/lockedin/apps/server/internal/testdb"
)

const secret = "0123456789abcdef0123456789abcdef0123456789abcdef"

func newAuth(t *testing.T) *Service {
	t.Helper()
	return NewService(testdb.New(t), testdb.Redis(t), secret)
}

func TestRegisterLoginLogout(t *testing.T) {
	ctx := context.Background()
	a := newAuth(t)
	u, err := a.Register(ctx, RegisterInput{Email: "Andi@Tepati.test", Password: "belajar-terus", DisplayName: " Andi "})
	if err != nil {
		t.Fatal(err)
	}
	if u.Email != "andi@tepati.test" || u.DisplayName != "Andi" || u.Locale != "id" || u.Timezone != "Asia/Jakarta" {
		t.Fatalf("normalised user: %+v", u)
	}
	if _, err := a.Register(ctx, RegisterInput{Email: "andi@tepati.test", Password: "belajar-terus", DisplayName: "Lain"}); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("duplicate email: %v", err)
	}

	if _, _, err := a.Login(ctx, "andi@tepati.test", "salah-password", "10.0.0.1"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password: %v", err)
	}
	if _, _, err := a.Login(ctx, "nobody@tepati.test", "belajar-terus", "10.0.0.1"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("unknown email gets the same error: %v", err)
	}
	_, token, err := a.Login(ctx, "ANDI@tepati.test", "belajar-terus", "10.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	id, err := a.Sessions().Lookup(ctx, token)
	if err != nil || id != u.ID {
		t.Fatalf("session lookup: %v %v", id, err)
	}
	if err := a.Logout(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Sessions().Lookup(ctx, token); !errors.Is(err, ErrNoSession) {
		t.Fatalf("logged-out token must be dead: %v", err)
	}
}

func TestLoginRateLimit(t *testing.T) {
	ctx := context.Background()
	a := newAuth(t)
	_, _ = a.Register(ctx, RegisterInput{Email: "bima@tepati.test", Password: "belajar-terus", DisplayName: "Bima"})
	for i := range loginLimit {
		if _, _, err := a.Login(ctx, "bima@tepati.test", "salah-salah", "10.0.0.9"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("attempt %d: %v", i+1, err)
		}
	}
	if _, _, err := a.Login(ctx, "bima@tepati.test", "belajar-terus", "10.0.0.9"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("11th attempt from the same IP must be limited, even with the right password: %v", err)
	}
	// Per-email limit applies from another IP too.
	if _, _, err := a.Login(ctx, "bima@tepati.test", "belajar-terus", "10.0.0.10"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("per-email limit: %v", err)
	}
}

// A test suite (or a busy office behind one IP) can raise the budget outside production; the
// default and the production cap stay at 10 (config.go).
func TestLoginRateLimitCanBeRaised(t *testing.T) {
	ctx := context.Background()
	a := newAuth(t).WithLoginLimit(25)
	_, _ = a.Register(ctx, RegisterInput{Email: "dewi@tepati.test", Password: "belajar-terus", DisplayName: "Dewi"})
	for i := range 25 {
		if _, _, err := a.Login(ctx, "dewi@tepati.test", "salah-salah", "10.0.0.20"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("attempt %d: %v", i+1, err)
		}
	}
	if _, _, err := a.Login(ctx, "dewi@tepati.test", "belajar-terus", "10.0.0.20"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("26th attempt must be limited: %v", err)
	}
}

func TestRequireUserMiddleware(t *testing.T) {
	ctx := context.Background()
	a := newAuth(t)
	u, _ := a.Register(ctx, RegisterInput{Email: "citra@tepati.test", Password: "belajar-terus", DisplayName: "Citra"})
	_, token, err := a.Login(ctx, "citra@tepati.test", "belajar-terus", "10.0.0.2")
	if err != nil {
		t.Fatal(err)
	}

	app := fiber.New()
	app.Use(RequireUser(a.Sessions()))
	app.All("/me", func(c fiber.Ctx) error { return c.SendString(UserID(c).String()) })

	do := func(method string, cookie, csrf string) (int, string) {
		req := httptest.NewRequest(method, "/me", nil)
		if cookie != "" {
			req.AddCookie(&http.Cookie{Name: SessionCookie, Value: cookie})
		}
		if csrf != "" {
			req.Header.Set(CSRFHeader, csrf)
		}
		res, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		body := new(strings.Builder)
		buf := make([]byte, 256)
		n, _ := res.Body.Read(buf)
		body.Write(buf[:n])
		return res.StatusCode, body.String()
	}

	if code, _ := do("GET", "", ""); code != 401 {
		t.Fatalf("no cookie: %d", code)
	}
	if code, _ := do("GET", "forged", ""); code != 401 {
		t.Fatalf("unknown token: %d", code)
	}
	if code, body := do("GET", token, ""); code != 200 || body != u.ID.String() {
		t.Fatalf("GET with session: %d %q", code, body)
	}
	if code, _ := do("POST", token, ""); code != 403 {
		t.Fatalf("POST without CSRF: %d", code)
	}
	if code, _ := do("POST", token, a.Sessions().CSRFToken("other-session")); code != 403 {
		t.Fatalf("POST with another session's CSRF: %d", code)
	}
	if code, _ := do("POST", token, a.Sessions().CSRFToken(token)); code != 200 {
		t.Fatalf("POST with CSRF: %d", code)
	}
}
