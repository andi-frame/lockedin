// Package ctl holds the logic behind the tepatictl admin CLI, so it can be tested without a
// process: development seeds and the pact inspector.
package ctl

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/andi-frame/lockedin/apps/server/internal/auth"
	"github.com/andi-frame/lockedin/apps/server/internal/domain"
	"github.com/andi-frame/lockedin/apps/server/internal/service"
	"github.com/andi-frame/lockedin/apps/server/internal/store"
)

// SeedPassword is the password of every seeded user, so a developer can log in as them.
const SeedPassword = "tepati-seed-1234"

const (
	seedBackerEmail  = "seed-backer@tepati.test"
	seedDoerEmail    = "seed-doer@tepati.test"
	seedInviteeEmail = "seed-invitee@tepati.test"
	seedTimezone     = "Asia/Jakarta"
	overdueDays      = 3 // how long ago the overdue pact started
)

// Seeded describes what a seed run created.
type Seeded struct {
	Pact         store.Pact
	Pacts        []store.Pact // the today scenario: one pact per state
	Backer, Doer store.User
	InviteToken  string // invite scenario only
	InviteEmail  string
}

// SeedOverdue builds an active pact that started three days ago, so several check-ins are past
// their deadline and nobody submitted anything. The worker, running on the real clock, settles
// them (PLAN 3.1's check). It goes through the normal service flow with a clock set in the
// past, because only the service may move state (AGENTS.md invariant 3).
func SeedOverdue(ctx context.Context, st *store.Store, now time.Time) (Seeded, error) {
	loc, err := time.LoadLocation(seedTimezone)
	if err != nil {
		return Seeded{}, err
	}
	backer, doer, err := seedUsers(ctx, st)
	if err != nil {
		return Seeded{}, err
	}
	start := localDate(now, loc, -overdueDays)
	clock := domain.NewFakeClock(noon(start, loc).Add(-24 * time.Hour))
	svc := service.New(st, clock)

	p, err := svc.CreateDraft(ctx, backer.ID, service.DraftInput{Title: "Seed: UTBK, terlambat", Terms: seedTerms(backer.ID, start, start.AddDays(13))})
	if err != nil {
		return Seeded{}, fmt.Errorf("draft: %w", err)
	}
	token, err := svc.Propose(ctx, backer.ID, p.ID, nil)
	if err != nil {
		return Seeded{}, fmt.Errorf("propose: %w", err)
	}
	if p, err = svc.JoinByInvite(ctx, doer.ID, token); err != nil {
		return Seeded{}, fmt.Errorf("join: %w", err)
	}
	for _, signer := range []store.User{backer, doer} {
		if _, err := svc.Accept(ctx, signer.ID, p.ID, p.TermsHash, signer.DisplayName); err != nil {
			return Seeded{}, fmt.Errorf("accept (%s): %w", signer.DisplayName, err)
		}
	}
	clock.Set(noon(start, loc))
	if _, err := svc.ActivateDuePacts(ctx); err != nil {
		return Seeded{}, fmt.Errorf("activate: %w", err)
	}
	if p, err = st.GetPact(ctx, p.ID); err != nil {
		return Seeded{}, err
	}
	return Seeded{Pact: p, Backer: backer, Doer: doer}, nil
}

// SeedInvite leaves a proposed pact with an open invite addressed to a seed email, which is
// what PLAN 3.2's invite email is built from.
func SeedInvite(ctx context.Context, st *store.Store, now time.Time) (Seeded, error) {
	loc, err := time.LoadLocation(seedTimezone)
	if err != nil {
		return Seeded{}, err
	}
	backer, doer, err := seedUsers(ctx, st)
	if err != nil {
		return Seeded{}, err
	}
	start := localDate(now, loc, 7)
	svc := service.New(st, domain.NewFakeClock(now))
	p, err := svc.CreateDraft(ctx, backer.ID, service.DraftInput{Title: "Seed: UTBK, undangan", Terms: seedTerms(backer.ID, start, start.AddDays(13))})
	if err != nil {
		return Seeded{}, fmt.Errorf("draft: %w", err)
	}
	email := seedInviteeEmail
	token, err := svc.Propose(ctx, backer.ID, p.ID, &email)
	if err != nil {
		return Seeded{}, fmt.Errorf("propose: %w", err)
	}
	if p, err = st.GetPact(ctx, p.ID); err != nil {
		return Seeded{}, err
	}
	return Seeded{Pact: p, Backer: backer, Doer: doer, InviteToken: token, InviteEmail: email}, nil
}

// todayStates are the pacts SeedToday builds, one per state the Today screen draws for the doer's
// check-in dated today.
var todayStates = []struct {
	title         string
	backerCommits bool
	cutoff        string
	grace         int
	doerSubmits   bool
	backerSubmits bool
	approve       bool
	doerEvidence  domain.Evidence // the doer's proof rules; zero means anything is enough
}{
	{title: "Today: open", backerCommits: true, cutoff: "23:59", grace: 30},
	// Open like the first, but with evidence rules and a later deadline (so it sorts after it): the
	// proof editor's e2e needs a minimum of words and a ready attachment before it may submit.
	{title: "Today: rules", backerCommits: true, cutoff: "23:59", grace: 60, doerEvidence: domain.Evidence{MinAttachments: 1, MinWords: 10}},
	{title: "Today: submitted", backerCommits: true, cutoff: "23:59", grace: 30, doerSubmits: true, backerSubmits: true},
	{title: "Today: approved", backerCommits: true, cutoff: "23:59", grace: 30, doerSubmits: true, approve: true},
	// A cutoff at midnight with no grace has always passed, so the sweep below turns today's
	// check-in into `missed` at once. The backer does not commit here, so the passbook has one line.
	{title: "Today: missed", backerCommits: false, cutoff: "00:00", grace: 0},
}

const todayProof = `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Selesai latihan soal hari ini."}]}]}`

// SeedToday gives one doer a Today screen with every state at once: an open check-in (and a second with evidence rules), a submitted
// one (with the backer's own proof waiting for the doer to review), an approved one and a missed
// one, each in its own active pact. Users are new on every run (named after tag) so the 10-pact
// limit is never reached and a test can run it as often as it likes. Everything goes through the
// service with a clock, and the real-time sweep makes the missed day, so the states are the ones
// the rules produce (AGENTS.md invariant 3).
func SeedToday(ctx context.Context, st *store.Store, now time.Time, tag string) (Seeded, error) {
	loc, err := time.LoadLocation(seedTimezone)
	if err != nil {
		return Seeded{}, err
	}
	backer, err := seedUser(ctx, st, fmt.Sprintf("today-%s-backer@tepati.test", tag), "Andi (today)")
	if err != nil {
		return Seeded{}, err
	}
	doer, err := seedUser(ctx, st, fmt.Sprintf("today-%s-doer@tepati.test", tag), "Bima (today)")
	if err != nil {
		return Seeded{}, err
	}
	today := localDate(now, loc, 0)
	clock := domain.NewFakeClock(noon(today, loc).Add(-24 * time.Hour))
	svc := service.New(st, clock)

	pacts := make([]store.Pact, 0, len(todayStates))
	for _, sc := range todayStates {
		terms := seedTerms(backer.ID, today, today.AddDays(13))
		terms.CutoffLocalTime, terms.GraceMinutes, terms.BackerCommits = sc.cutoff, sc.grace, sc.backerCommits
		for id, m := range terms.Members {
			m.Evidence = domain.Evidence{}
			if m.Role == domain.RoleDoer {
				m.Evidence = sc.doerEvidence
			}
			if m.Role == domain.RoleBacker && !sc.backerCommits {
				m.Schedule = nil
			}
			terms.Members[id] = m
		}
		p, err := svc.CreateDraft(ctx, backer.ID, service.DraftInput{Title: sc.title, Terms: terms})
		if err != nil {
			return Seeded{}, fmt.Errorf("%s: draft: %w", sc.title, err)
		}
		token, err := svc.Propose(ctx, backer.ID, p.ID, nil)
		if err != nil {
			return Seeded{}, fmt.Errorf("%s: propose: %w", sc.title, err)
		}
		if p, err = svc.JoinByInvite(ctx, doer.ID, token); err != nil {
			return Seeded{}, fmt.Errorf("%s: join: %w", sc.title, err)
		}
		for _, signer := range []store.User{backer, doer} {
			if _, err := svc.Accept(ctx, signer.ID, p.ID, p.TermsHash, signer.DisplayName); err != nil {
				return Seeded{}, fmt.Errorf("%s: accept (%s): %w", sc.title, signer.DisplayName, err)
			}
		}
		pacts = append(pacts, p)
	}

	clock.Set(now)
	if _, err := svc.ActivateDuePacts(ctx); err != nil {
		return Seeded{}, fmt.Errorf("activate: %w", err)
	}
	day := today.Time()
	for i, sc := range todayStates {
		rows, err := st.ListCheckInsForPact(ctx, store.ListCheckInsForPactParams{PactID: pacts[i].ID, FromDate: day, ToDate: day})
		if err != nil {
			return Seeded{}, err
		}
		proof := service.ProofInput{BodyDoc: []byte(todayProof)}
		for _, ci := range rows {
			switch {
			case ci.MemberID == doer.ID && sc.doerSubmits:
				if _, err := svc.SubmitProof(ctx, doer.ID, ci.ID, proof); err != nil {
					return Seeded{}, fmt.Errorf("%s: doer submit: %w", sc.title, err)
				}
				if sc.approve {
					if _, err := svc.Approve(ctx, backer.ID, ci.ID); err != nil {
						return Seeded{}, fmt.Errorf("%s: approve: %w", sc.title, err)
					}
				}
			case ci.MemberID == backer.ID && sc.backerSubmits:
				if _, err := svc.SubmitProof(ctx, backer.ID, ci.ID, proof); err != nil {
					return Seeded{}, fmt.Errorf("%s: backer submit: %w", sc.title, err)
				}
			}
		}
	}
	if _, err := svc.SweepDeadlines(ctx, 500); err != nil {
		return Seeded{}, fmt.Errorf("sweep: %w", err)
	}
	for i := range pacts {
		if pacts[i], err = st.GetPact(ctx, pacts[i].ID); err != nil {
			return Seeded{}, err
		}
	}
	return Seeded{Pact: pacts[0], Pacts: pacts, Backer: backer, Doer: doer}, nil
}

func seedUsers(ctx context.Context, st *store.Store) (backer, doer store.User, err error) {
	if backer, err = seedUser(ctx, st, seedBackerEmail, "Andi (seed)"); err != nil {
		return
	}
	doer, err = seedUser(ctx, st, seedDoerEmail, "Bima (seed)")
	return
}

// seedUser returns the user with this email, creating it on the first run.
func seedUser(ctx context.Context, st *store.Store, email, name string) (store.User, error) {
	u, err := st.GetUserByEmail(ctx, email)
	if err == nil {
		return u, nil
	}
	if !store.IsNoRows(err) {
		return store.User{}, err
	}
	hash, err := auth.HashPassword(SeedPassword)
	if err != nil {
		return store.User{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return store.User{}, err
	}
	u, err = st.CreateUser(ctx, store.CreateUserParams{ID: id, Email: email, PasswordHash: hash, DisplayName: name, Locale: "id", Timezone: seedTimezone})
	if err != nil {
		return store.User{}, errors.Join(fmt.Errorf("create seed user %s", email), err)
	}
	return u, nil
}

// seedTerms is the PLAN worked example, but every day of the week is a check-in day so an
// overdue one always exists.
func seedTerms(backer uuid.UUID, startsOn, endsOn domain.Date) domain.Terms {
	potCap := int64(1500)
	everyDay := []int{1, 2, 3, 4, 5, 6, 7}
	return domain.Terms{
		Version: 1, Timezone: seedTimezone, StartsOn: startsOn, EndsOn: endsOn,
		CutoffLocalTime: "23:59", GraceMinutes: 30, CoinRateIDR: 1000,
		InitialPot: 1000, PotFloor: 0, PotCap: &potCap,
		ReviewWindowHours: 24, DisputeWindowHours: 24, DisputeResolutionHours: 48, OverrideWindowHours: 48,
		MaxOverrides: 3, BackerCommits: true,
		Members: map[uuid.UUID]domain.MemberTerms{
			backer:   {Role: domain.RoleBacker, Commitment: "Belajar Kalkulus 2 jam", Schedule: everyDay, PenaltyPerMiss: 50, RestDays: 2},
			uuid.Nil: {Role: domain.RoleDoer, Commitment: "Latihan soal UTBK 50 soal", Schedule: everyDay, PenaltyPerMiss: 50, RestDays: 2},
		},
	}
}

// localDate is the calendar date in loc, plus offset days.
func localDate(t time.Time, loc *time.Location, offset int) domain.Date {
	l := t.In(loc).AddDate(0, 0, offset)
	return domain.NewDate(l.Year(), l.Month(), l.Day())
}

func noon(d domain.Date, loc *time.Location) time.Time {
	s := d.String()
	t, _ := time.ParseInLocation("2006-01-02 15:04", s+" 12:00", loc)
	return t
}
