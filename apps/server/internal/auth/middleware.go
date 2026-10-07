package auth

import (
	"errors"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

const localsUser = "auth.user_id"

// RequireUser rejects requests without a valid session cookie (401) and stores the
// user id for handlers. Unsafe methods also need a CSRF header bound to the session.
func RequireUser(s *Sessions) fiber.Handler {
	return func(c fiber.Ctx) error {
		token := c.Cookies(SessionCookie)
		user, err := s.Lookup(c.Context(), token)
		if errors.Is(err, ErrNoSession) {
			return fiber.NewError(fiber.StatusUnauthorized, "auth.unauthenticated")
		}
		if err != nil {
			return err
		}
		if !safeMethod(c.Method()) && !s.ValidCSRF(token, c.Get(CSRFHeader)) {
			return fiber.NewError(fiber.StatusForbidden, "auth.csrf")
		}
		c.Locals(localsUser, user)
		return c.Next()
	}
}

// UserID returns the authenticated user set by RequireUser.
func UserID(c fiber.Ctx) uuid.UUID {
	id, _ := c.Locals(localsUser).(uuid.UUID)
	return id
}

func safeMethod(m string) bool {
	return m == fiber.MethodGet || m == fiber.MethodHead || m == fiber.MethodOptions
}

// SetSessionCookies writes the HttpOnly session cookie and the readable CSRF cookie
// the web app copies into the X-CSRF-Token header.
func SetSessionCookies(c fiber.Ctx, s *Sessions, token string, secure bool) {
	maxAge := int(SessionTTL.Seconds())
	c.Cookie(&fiber.Cookie{Name: SessionCookie, Value: token, Path: "/", MaxAge: maxAge, HTTPOnly: true, Secure: secure, SameSite: fiber.CookieSameSiteLaxMode})
	c.Cookie(&fiber.Cookie{Name: CSRFCookie, Value: s.CSRFToken(token), Path: "/", MaxAge: maxAge, HTTPOnly: false, Secure: secure, SameSite: fiber.CookieSameSiteLaxMode})
}

func ClearSessionCookies(c fiber.Ctx, secure bool) {
	for _, name := range []string{SessionCookie, CSRFCookie} {
		c.Cookie(&fiber.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1, HTTPOnly: name == SessionCookie, Secure: secure, SameSite: fiber.CookieSameSiteLaxMode})
	}
}
