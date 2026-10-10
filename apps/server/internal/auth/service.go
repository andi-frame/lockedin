package auth

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"

	"github.com/andi-frame/lockedin/apps/server/internal/domain"
	"github.com/andi-frame/lockedin/apps/server/internal/store"
)

var (
	ErrInvalidCredentials = &domain.Error{Code: "auth.invalid_credentials", Msg: "email or password is wrong"}
	ErrEmailTaken         = &domain.Error{Code: "auth.email_taken", Msg: "this email is already registered"}
	ErrInvalidEmail       = &domain.Error{Code: "auth.invalid_email", Msg: "enter a valid email address"}
	ErrInvalidName        = &domain.Error{Code: "auth.invalid_name", Msg: "display name must be 1 to 80 characters"}
	ErrWrongPassword      = &domain.Error{Code: "auth.wrong_password", Msg: "the current password is wrong"}
)

// Login limits: 10 attempts per minute per IP, and per email (ARCHITECTURE §3).
const (
	loginLimit  = 10
	loginWindow = time.Minute
)

type Service struct {
	st       *store.Store
	rdb      *redis.Client
	sessions *Sessions
	login    *Limiter
	// dummyHash keeps login timing similar whether or not the email exists.
	dummyHash string
}

func NewService(st *store.Store, rdb *redis.Client, sessionSecret string) *Service {
	dummy, _ := HashPassword("timing-equaliser-password")
	return &Service{
		st:        st,
		rdb:       rdb,
		sessions:  NewSessions(rdb, sessionSecret),
		login:     NewLimiter(rdb, "login", loginLimit, loginWindow),
		dummyHash: dummy,
	}
}

// WithLoginLimit sets how many login attempts one IP, and one email, may make a minute. The
// default (10) is the production value; config.go lets a test suite raise it outside production.
func (s *Service) WithLoginLimit(n int64) *Service {
	s.login = NewLimiter(s.rdb, "login", n, loginWindow)
	return s
}

func (s *Service) Sessions() *Sessions { return s.sessions }

type RegisterInput struct {
	Email, Password, DisplayName, Locale, Timezone string
}

func (s *Service) Register(ctx context.Context, in RegisterInput) (store.User, error) {
	email, err := normaliseEmail(in.Email)
	if err != nil {
		return store.User{}, err
	}
	name := strings.TrimSpace(in.DisplayName)
	if n := len([]rune(name)); n < 1 || n > 80 {
		return store.User{}, ErrInvalidName
	}
	hash, err := HashPassword(in.Password)
	if err != nil {
		return store.User{}, err
	}
	locale := in.Locale
	if locale != "en" {
		locale = "id"
	}
	tz := in.Timezone
	if _, err := time.LoadLocation(tz); err != nil || tz == "" {
		tz = "Asia/Jakarta"
	}
	u, err := s.st.CreateUser(ctx, store.CreateUserParams{
		ID: uuid.Must(uuid.NewV7()), Email: email, PasswordHash: hash, DisplayName: name, Locale: locale, Timezone: tz,
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return store.User{}, ErrEmailTaken
	}
	return u, err
}

// Login verifies credentials and returns a new session token.
func (s *Service) Login(ctx context.Context, email, password, ip string) (store.User, string, error) {
	email, err := normaliseEmail(email)
	if err != nil {
		return store.User{}, "", ErrInvalidCredentials
	}
	if err := s.login.Allow(ctx, "ip:"+ip, "email:"+email); err != nil {
		return store.User{}, "", err
	}
	u, err := s.st.GetUserByEmail(ctx, email)
	if store.IsNoRows(err) {
		_, _ = VerifyPassword(password, s.dummyHash)
		return store.User{}, "", ErrInvalidCredentials
	}
	if err != nil {
		return store.User{}, "", err
	}
	ok, err := VerifyPassword(password, u.PasswordHash)
	if err != nil || !ok {
		return store.User{}, "", ErrInvalidCredentials
	}
	token, err := s.sessions.Create(ctx, u.ID)
	if err != nil {
		return store.User{}, "", err
	}
	return u, token, nil
}

// ChangePassword sets a new password after checking the current one, then ends every session of
// the user except `keepToken` (the one asking), so a stolen session or a forgotten device does not
// survive the change. Attempts are limited per user with the login limiter: a stolen session must
// not be able to guess the current password.
func (s *Service) ChangePassword(ctx context.Context, user uuid.UUID, keepToken, current, next string) error {
	if err := s.login.Allow(ctx, "pw:"+user.String()); err != nil {
		return err
	}
	u, err := s.st.GetUser(ctx, user)
	if err != nil {
		return err
	}
	ok, err := VerifyPassword(current, u.PasswordHash)
	if err != nil || !ok {
		return ErrWrongPassword
	}
	hash, err := HashPassword(next)
	if err != nil {
		return err
	}
	if err := s.st.UpdateUserPassword(ctx, store.UpdateUserPasswordParams{ID: user, PasswordHash: hash}); err != nil {
		return err
	}
	return s.sessions.RevokeOthers(ctx, user, keepToken)
}

func (s *Service) Logout(ctx context.Context, token string) error {
	return s.sessions.Delete(ctx, token)
}

func normaliseEmail(raw string) (string, error) {
	e := strings.ToLower(strings.TrimSpace(raw))
	addr, err := mail.ParseAddress(e)
	if err != nil || addr.Address != e || len(e) > 254 {
		return "", ErrInvalidEmail
	}
	return e, nil
}
