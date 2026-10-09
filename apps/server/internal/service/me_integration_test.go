//go:build integration

package service_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/andi-frame/lockedin/apps/server/internal/domain"
	"github.com/andi-frame/lockedin/apps/server/internal/service"
	"github.com/andi-frame/lockedin/apps/server/internal/store"
	"github.com/andi-frame/lockedin/apps/server/internal/testdb"
)

func ptr[T any](v T) *T { return &v }

func newUser(t *testing.T, st *store.Store, email, name string) store.User {
	t.Helper()
	u, err := st.CreateUser(context.Background(), store.CreateUserParams{ID: uuid.Must(uuid.NewV7()), Email: email, PasswordHash: "x", DisplayName: name, Locale: "id", Timezone: "Asia/Jakarta"})
	if err != nil {
		t.Fatal(err)
	}
	return u
}

// PLAN 9.4: only the fields that are sent change, and bad values are refused with the contract's codes.
func TestUpdateMe(t *testing.T) {
	st := testdb.New(t)
	svc := service.New(st, domain.NewFakeClock(time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)))
	ctx := context.Background()
	u := newUser(t, st, "sari@tepati.test", "Sari")

	got, err := svc.UpdateMe(ctx, u.ID, service.ProfileUpdate{DisplayName: ptr("  Sari Dewi  ")})
	if err != nil {
		t.Fatal(err)
	}
	if got.DisplayName != "Sari Dewi" || got.Locale != "id" || got.Timezone != "Asia/Jakarta" || len(got.EmailOff) != 0 {
		t.Errorf("name only: %+v", got)
	}

	got, err = svc.UpdateMe(ctx, u.ID, service.ProfileUpdate{Locale: ptr("en"), Timezone: ptr("Asia/Makassar"), EmailOff: ptr([]string{"terms_signed", "proof_rejected", "terms_signed"})})
	if err != nil {
		t.Fatal(err)
	}
	if got.DisplayName != "Sari Dewi" || got.Locale != "en" || got.Timezone != "Asia/Makassar" {
		t.Errorf("rest: %+v", got)
	}
	if want := []string{"proof_rejected", "terms_signed"}; len(got.EmailOff) != 2 || got.EmailOff[0] != want[0] || got.EmailOff[1] != want[1] {
		t.Errorf("email_off = %v, want %v", got.EmailOff, want)
	}

	// An empty list switches everything back on; leaving the field out keeps the list.
	if got, err = svc.UpdateMe(ctx, u.ID, service.ProfileUpdate{DisplayName: ptr("Sari")}); err != nil || len(got.EmailOff) != 2 {
		t.Fatalf("a request without email_kinds_off must keep the list: %v %v", got.EmailOff, err)
	}
	if got, err = svc.UpdateMe(ctx, u.ID, service.ProfileUpdate{EmailOff: ptr([]string{})}); err != nil || len(got.EmailOff) != 0 {
		t.Fatalf("an empty list must clear it: %v %v", got.EmailOff, err)
	}

	for name, in := range map[string]service.ProfileUpdate{
		"empty name":        {DisplayName: ptr("   ")},
		"name over 80":      {DisplayName: ptr(string(make([]rune, 81)))},
		"unknown locale":    {Locale: ptr("fr")},
		"unknown time zone": {Timezone: ptr("Mars/Olympus")},
		"mandatory kind":    {EmailOff: ptr([]string{"dispute_opened"})},
	} {
		if _, err := svc.UpdateMe(ctx, u.ID, in); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
	if _, err := svc.UpdateMe(ctx, u.ID, service.ProfileUpdate{DisplayName: ptr("")}); domain.CodeOf(err) != "auth.invalid_name" {
		t.Errorf("code for an empty name = %q", domain.CodeOf(err))
	}
	if _, err := svc.UpdateMe(ctx, u.ID, service.ProfileUpdate{Locale: ptr("fr")}); domain.CodeOf(err) != "validation.failed" {
		t.Errorf("code for a bad locale = %q", domain.CodeOf(err))
	}
	// Nothing above may have changed the account.
	if after, _ := svc.Me(ctx, u.ID); after.DisplayName != "Sari" || after.Locale != "en" {
		t.Errorf("refused updates changed the account: %+v", after)
	}
}

// PLAN 9.4: a switched-off kind is claimed (so it is never retried) but produces no email; kinds
// that cannot be switched off are mailed whatever the list says.
func TestEmailClaimsHonourPreferences(t *testing.T) {
	st := testdb.New(t)
	svc := service.New(st, domain.NewFakeClock(time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)))
	ctx := context.Background()
	backer, doer := newUser(t, st, "andi@tepati.test", "Andi"), newUser(t, st, "bima@tepati.test", "Bima")
	pact, err := svc.CreateDraft(ctx, backer.ID, service.DraftInput{Title: "UTBK", Terms: termsFor(backer.ID)})
	if err != nil {
		t.Fatal(err)
	}
	notify := func(user store.User, kind string) int64 {
		payload, _ := json.Marshal(service.Notification{UserID: user.ID, Kind: kind, PactID: pact.ID})
		id, err := st.InsertNotification(ctx, store.InsertNotificationParams{UserID: user.ID, Kind: kind, Payload: payload})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	// Even a list written around the API (an old row, a manual fix) cannot silence a mandatory kind.
	if _, err := st.UpdateUserProfile(ctx, store.UpdateUserProfileParams{ID: doer.ID, EmailOff: []string{"proof_rejected", "proof_submitted", "dispute_opened"}}); err != nil {
		t.Fatal(err)
	}

	if _, ok, err := svc.ClaimNotificationEmail(ctx, notify(doer, "proof_rejected")); err != nil || ok {
		t.Errorf("switched-off immediate kind: ok=%v err=%v, want no email", ok, err)
	}
	id := notify(doer, "dispute_opened")
	if n, ok, err := svc.ClaimNotificationEmail(ctx, id); err != nil || !ok || n.Kind != "dispute_opened" {
		t.Errorf("a kind that cannot be switched off must still be mailed: %+v ok=%v err=%v", n, ok, err)
	}
	if _, ok, _ := svc.ClaimNotificationEmail(ctx, notify(backer, "proof_rejected")); !ok {
		t.Error("a user with no list must be mailed")
	}

	notify(doer, "proof_submitted")
	if _, ok, err := svc.ClaimDigestEmail(ctx, doer.ID, pact.ID, "proof_submitted"); err != nil || ok {
		t.Errorf("switched-off digest kind: ok=%v err=%v, want no email", ok, err)
	}
	// Claimed means handled: switching the kind back on does not resurrect old notifications.
	if _, err := st.UpdateUserProfile(ctx, store.UpdateUserProfileParams{ID: doer.ID, EmailOff: []string{}}); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := svc.ClaimDigestEmail(ctx, doer.ID, pact.ID, "proof_submitted"); ok {
		t.Error("an old notification was mailed after the kind was switched back on")
	}
	notify(doer, "proof_submitted")
	if d, ok, err := svc.ClaimDigestEmail(ctx, doer.ID, pact.ID, "proof_submitted"); err != nil || !ok || d.Count != 1 {
		t.Errorf("after switching on, a new notification must be mailed: %+v ok=%v err=%v", d, ok, err)
	}
}

func termsFor(backer uuid.UUID) domain.Terms {
	return domain.Terms{
		Version: 1, Timezone: "Asia/Jakarta",
		StartsOn: domain.MustDate("2026-11-02"), EndsOn: domain.MustDate("2026-11-29"),
		CutoffLocalTime: "23:59", GraceMinutes: 30, CoinRateIDR: 1000,
		InitialPot: 1000, PotFloor: 0,
		ReviewWindowHours: 24, DisputeWindowHours: 24, DisputeResolutionHours: 48, OverrideWindowHours: 48,
		MaxOverrides: 3,
		Members: map[uuid.UUID]domain.MemberTerms{
			backer:   {Role: domain.RoleBacker, PenaltyPerMiss: 50},
			uuid.Nil: {Role: domain.RoleDoer, Commitment: "Latihan soal UTBK 50 soal", Schedule: []int{1, 2, 3, 4, 5, 6}, PenaltyPerMiss: 50, RestDays: 2},
		},
	}
}
