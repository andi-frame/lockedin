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

// PLAN 9.9: changing the password needs the current one, ends every other session and keeps the
// one it was asked from, and a wrong or weak attempt changes nothing.
func TestChangePassword(t *testing.T) {
	ctx := context.Background()
	a := newAuth(t)
	u, err := a.Register(ctx, RegisterInput{Email: "sari@tepati.test", Password: "belajar-terus", DisplayName: "Sari"})
	if err != nil {
		t.Fatal(err)
	}
	_, here, err := a.Login(ctx, "sari@tepati.test", "belajar-terus", "10.0.1.1")
	if err != nil {
		t.Fatal(err)
	}
	_, phone, err := a.Login(ctx, "sari@tepati.test", "belajar-terus", "10.0.1.1")
	if err != nil {
		t.Fatal(err)
	}
	_, other, err := a.Login(ctx, "sari@tepati.test", "belajar-terus", "10.0.1.1")
	if err != nil {
		t.Fatal(err)
	}
	alive := func(token string) bool {
		_, err := a.Sessions().Lookup(ctx, token)
		return err == nil
	}

	// A wrong current password, and a new one that is too short, change nothing.
	if err := a.ChangePassword(ctx, u.ID, here, "bukan-yang-ini", "kata-sandi-baru-1"); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("wrong current password: %v", err)
	}
	if err := a.ChangePassword(ctx, u.ID, here, "belajar-terus", "pendek"); !errors.Is(err, ErrWeakPassword) {
		t.Fatalf("weak new password: %v", err)
	}
	if !alive(here) || !alive(phone) || !alive(other) {
		t.Fatal("a refused change must leave every session alone")
	}
	if _, _, err := a.Login(ctx, "sari@tepati.test", "belajar-terus", "10.0.1.2"); err != nil {
		t.Fatalf("the old password must still work after refused changes: %v", err)
	}

	if err := a.ChangePassword(ctx, u.ID, here, "belajar-terus", "kata-sandi-baru-1"); err != nil {
		t.Fatal(err)
	}
	if !alive(here) {
		t.Error("the session the change was made from must stay signed in")
	}
	if alive(phone) || alive(other) {
		t.Error("every other session must be ended")
	}
	if _, _, err := a.Login(ctx, "sari@tepati.test", "belajar-terus", "10.0.1.3"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("the old password must stop working: %v", err)
	}
	if _, _, err := a.Login(ctx, "sari@tepati.test", "kata-sandi-baru-1", "10.0.1.3"); err != nil {
		t.Errorf("the new password must work: %v", err)
	}
}

// A stolen session must not be able to guess the current password: attempts are limited per user.
func TestChangePasswordIsRateLimited(t *testing.T) {
	ctx := context.Background()
	a := newAuth(t)
	u, _ := a.Register(ctx, RegisterInput{Email: "bima2@tepati.test", Password: "belajar-terus", DisplayName: "Bima"})
	_, token, _ := a.Login(ctx, "bima2@tepati.test", "belajar-terus", "10.0.2.1")
	for i := range loginLimit {
		if err := a.ChangePassword(ctx, u.ID, token, "salah-salah", "kata-sandi-baru-1"); !errors.Is(err, ErrWrongPassword) {
			t.Fatalf("attempt %d: %v", i+1, err)
		}
	}
	if err := a.ChangePassword(ctx, u.ID, token, "belajar-terus", "kata-sandi-baru-1"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("after %d wrong attempts even the right password must wait: %v", loginLimit, err)
	}
}

// Logging out removes the session from the user's index too, so the index does not grow forever.
func TestLogoutLeavesTheOtherSessionsRevocable(t *testing.T) {
	ctx := context.Background()
	a := newAuth(t)
	u, _ := a.Register(ctx, RegisterInput{Email: "dewi@tepati.test", Password: "belajar-terus", DisplayName: "Dewi"})
	_, first, _ := a.Login(ctx, "dewi@tepati.test", "belajar-terus", "10.0.3.1")
	_, second, _ := a.Login(ctx, "dewi@tepati.test", "belajar-terus", "10.0.3.1")
	if err := a.Logout(ctx, first); err != nil {
		t.Fatal(err)
	}
	if n, _ := a.rdb.SCard(ctx, "usess:"+u.ID.String()).Result(); n != 1 {
		t.Fatalf("the user's index holds %d sessions after one logout, want 1", n)
	}
	if err := a.Sessions().RevokeOthers(ctx, u.ID, "nobody-in-particular"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Sessions().Lookup(ctx, second); !errors.Is(err, ErrNoSession) {
		t.Fatal("RevokeOthers must end every session but the one it is told to keep")
	}
}
