package service

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/andi-frame/lockedin/apps/server/internal/domain"
	"github.com/andi-frame/lockedin/apps/server/internal/store"
)

var (
	// ErrInvalidName and ErrInvalidProfile carry the contract's codes: the name is also the
	// signature typed on terms, so it keeps the code that registration uses.
	ErrInvalidName    = &domain.Error{Code: "auth.invalid_name", Msg: "display name must be 1 to 80 characters"}
	ErrInvalidProfile = &domain.Error{Code: "validation.failed", Msg: "the request is not valid"}
)

// ProfileUpdate is what PATCH /me can change. A nil field is left as it is; EmailOff replaces the
// whole list when it is set (a non-nil empty list switches every email back on).
type ProfileUpdate struct {
	DisplayName *string
	Locale      *string
	Timezone    *string
	EmailOff    *[]string
}

// UpdateMe changes the signed-in user's account. A new name does not touch signatures already
// given on terms: those keep the name that was typed (pact_members.signature_name).
func (s *Service) UpdateMe(ctx context.Context, id uuid.UUID, in ProfileUpdate) (store.User, error) {
	p := store.UpdateUserProfileParams{ID: id}
	if in.DisplayName != nil {
		name := strings.TrimSpace(*in.DisplayName)
		if n := len([]rune(name)); n < 1 || n > 80 {
			return store.User{}, ErrInvalidName
		}
		p.DisplayName = &name
	}
	if in.Locale != nil {
		if *in.Locale != "id" && *in.Locale != "en" {
			return store.User{}, fmt.Errorf("%w: locale must be id or en", ErrInvalidProfile)
		}
		p.Locale = in.Locale
	}
	if in.Timezone != nil {
		if _, err := time.LoadLocation(*in.Timezone); err != nil || *in.Timezone == "" || *in.Timezone == "Local" {
			return store.User{}, fmt.Errorf("%w: unknown time zone %q", ErrInvalidProfile, *in.Timezone)
		}
		p.Timezone = in.Timezone
	}
	if in.EmailOff != nil {
		off, err := domain.NormaliseEmailOff(*in.EmailOff)
		if err != nil {
			return store.User{}, fmt.Errorf("%w: %v", ErrInvalidProfile, err)
		}
		p.EmailOff = off
	}
	u, err := s.st.UpdateUserProfile(ctx, p)
	if store.IsNoRows(err) {
		return store.User{}, ErrNotFound
	}
	return u, err
}

// wantsEmail reports whether a kind may be mailed to someone with this off-list. Kinds that
// cannot be switched off are mailed whatever the list says.
func wantsEmail(kind string, off []string) bool {
	return !domain.IsSwitchableEmailKind(kind) || !slices.Contains(off, kind)
}
