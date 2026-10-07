//go:build integration

package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/andi-frame/lockedin/apps/server/internal/domain"
	"github.com/andi-frame/lockedin/apps/server/internal/store"
	"github.com/andi-frame/lockedin/apps/server/internal/testdb"
)

type fixture struct {
	t      *testing.T
	ctx    context.Context
	st     *store.Store
	clock  *domain.FakeClock
	svc    *Service
	backer store.User
	doer   store.User
}

// Monday 2026-10-26 09:00 WIB, a week before the worked example starts.
var fixtureStart = time.Date(2026, 10, 26, 2, 0, 0, 0, time.UTC)

func newFixture(t *testing.T) *fixture {
	t.Helper()
	st := testdb.New(t)
	clock := domain.NewFakeClock(fixtureStart)
	f := &fixture{t: t, ctx: context.Background(), st: st, clock: clock, svc: New(st, clock)}
	f.backer = f.user("andi@tepati.test", "Andi")
	f.doer = f.user("bima@tepati.test", "Bima")
	return f
}

func (f *fixture) user(email, name string) store.User {
	f.t.Helper()
	u, err := f.st.CreateUser(f.ctx, store.CreateUserParams{
		ID: newID(), Email: email, PasswordHash: "x", DisplayName: name, Locale: "id", Timezone: "Asia/Jakarta",
	})
	if err != nil {
		f.t.Fatal(err)
	}
	return u
}

func i64(v int64) *int64 { return &v }

// workedTerms is the PLAN.md worked example; the doer slot is uuid.Nil until they join.
func workedTerms(backer uuid.UUID) domain.Terms {
	return domain.Terms{
		Version: 1, Timezone: "Asia/Jakarta",
		StartsOn: domain.MustDate("2026-11-02"), EndsOn: domain.MustDate("2026-11-29"),
		CutoffLocalTime: "23:59", GraceMinutes: 30, CoinRateIDR: 1000,
		InitialPot: 1000, PotFloor: 0, PotCap: i64(1500),
		ReviewWindowHours: 24, DisputeWindowHours: 24, DisputeResolutionHours: 48, OverrideWindowHours: 48,
		MaxOverrides: 3, BackerCommits: true,
		Members: map[uuid.UUID]domain.MemberTerms{
			backer:   {Role: domain.RoleBacker, Commitment: "Belajar Kalkulus 2 jam", Schedule: []int{1, 2, 3, 4, 5}, PenaltyPerMiss: 50, RestDays: 2},
			uuid.Nil: {Role: domain.RoleDoer, Commitment: "Latihan soal UTBK 50 soal", Schedule: []int{1, 2, 3, 4, 5, 6}, PenaltyPerMiss: 50, RestDays: 2},
		},
	}
}

func (f *fixture) must(err error) {
	f.t.Helper()
	if err != nil {
		f.t.Fatal(err)
	}
}

// scheduledPact runs the whole agreement flow and returns the scheduled pact.
func (f *fixture) scheduledPact() store.Pact {
	f.t.Helper()
	p, err := f.svc.CreateDraft(f.ctx, f.backer.ID, DraftInput{Title: "UTBK November", Terms: workedTerms(f.backer.ID)})
	f.must(err)
	token, err := f.svc.Propose(f.ctx, f.backer.ID, p.ID, nil)
	f.must(err)
	p, err = f.svc.JoinByInvite(f.ctx, f.doer.ID, token)
	f.must(err)
	_, err = f.svc.Accept(f.ctx, f.backer.ID, p.ID, p.TermsHash, "Andi")
	f.must(err)
	p, err = f.svc.Accept(f.ctx, f.doer.ID, p.ID, p.TermsHash, "bima")
	f.must(err)
	return p
}

func (f *fixture) balance(pactID uuid.UUID) int64 {
	f.t.Helper()
	b, err := f.st.PotBalance(f.ctx, pactID)
	f.must(err)
	return b
}
