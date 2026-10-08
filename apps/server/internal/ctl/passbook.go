package ctl

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/andi-frame/lockedin/apps/server/internal/domain"
	"github.com/andi-frame/lockedin/apps/server/internal/service"
	"github.com/andi-frame/lockedin/apps/server/internal/store"
)

// passbookDays is how many finished days the passbook scenario has behind it. With the misses
// below that is 33 lines, more than the pact page loads at once, so the page has a second page.
const passbookDays = 24

// SeedPassbook builds one active pact with passbookDays of history, then leaves today open for the
// doer and already sent by the backer. Time is walked forward a day at a time with a fake clock
// and every step goes through the service (AGENTS.md invariant 3), so the lines are the ones the
// rules print:
//
//	doer, day i (0 is the first day):  i%3==0 sent and approved; i==2 and i==5 rest; i==4 sent and
//	                                   rejected (a penalty once the dispute window closes); the rest missed
//	backer, day i:                     i%4==0 sent and approved; the rest missed
//
// The doer pays 50 a miss and the backer adds 20, so the passbook has debits, credits and a
// balance that moves both ways.
func SeedPassbook(ctx context.Context, st *store.Store, now time.Time, tag string) (Seeded, error) {
	loc, err := time.LoadLocation(seedTimezone)
	if err != nil {
		return Seeded{}, err
	}
	backer, err := seedUser(ctx, st, fmt.Sprintf("passbook-%s-backer@tepati.test", tag), "Andi (buku)")
	if err != nil {
		return Seeded{}, err
	}
	doer, err := seedUser(ctx, st, fmt.Sprintf("passbook-%s-doer@tepati.test", tag), "Bima (buku)")
	if err != nil {
		return Seeded{}, err
	}

	today := localDate(now, loc, 0)
	start := today.AddDays(-passbookDays)
	clock := domain.NewFakeClock(noon(start, loc).Add(-24 * time.Hour))
	svc := service.New(st, clock)

	terms := seedTerms(backer.ID, start, today.AddDays(1))
	terms.GraceMinutes = 0 // so the day is missed the minute the cutoff passes, which Advance relies on
	for id, m := range terms.Members {
		m.PenaltyPerMiss = 50
		if m.Role == domain.RoleBacker {
			m.PenaltyPerMiss = 20
		}
		terms.Members[id] = m
	}
	p, err := svc.CreateDraft(ctx, backer.ID, service.DraftInput{Title: "Passbook: history", Terms: terms})
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

	// Each day: the proofs and rests at 08:00, then the sweep at noon (it settles the day before).
	// Sweeping at a different hour than the actions keeps a 24-hour window from closing on the
	// exact instant it is checked.
	proof := service.ProofInput{BodyDoc: []byte(todayProof)}
	for i := 0; i < passbookDays; i++ {
		day := start.AddDays(i)
		clock.Set(noon(day, loc).Add(-4 * time.Hour))
		cis, err := checkInsOn(ctx, st, p.ID, day)
		if err != nil {
			return Seeded{}, err
		}
		for _, ci := range cis {
			if err := playPassbookDay(ctx, svc, i, ci, backer.ID, doer.ID); err != nil {
				return Seeded{}, fmt.Errorf("day %d: %w", i, err)
			}
		}
		clock.Set(noon(day, loc))
		if _, err := svc.SweepDeadlines(ctx, 500); err != nil {
			return Seeded{}, fmt.Errorf("sweep after day %d: %w", i, err)
		}
	}

	// Today: the backer has sent a proof, the doer has not, so one miss is left to print.
	clock.Set(now)
	if _, err := svc.SweepDeadlines(ctx, 500); err != nil {
		return Seeded{}, fmt.Errorf("sweep: %w", err)
	}
	cis, err := checkInsOn(ctx, st, p.ID, today)
	if err != nil {
		return Seeded{}, err
	}
	for _, ci := range cis {
		if ci.MemberID == backer.ID {
			if _, err := svc.SubmitProof(ctx, backer.ID, ci.ID, proof); err != nil {
				return Seeded{}, fmt.Errorf("backer submit today: %w", err)
			}
		}
	}
	if p, err = st.GetPact(ctx, p.ID); err != nil {
		return Seeded{}, err
	}
	return Seeded{Pact: p, Pacts: []store.Pact{p}, Backer: backer, Doer: doer}, nil
}

func playPassbookDay(ctx context.Context, svc *service.Service, i int, ci store.CheckIn, backer, doer uuid.UUID) error {
	proof := service.ProofInput{BodyDoc: []byte(todayProof)}
	if ci.MemberID == doer {
		switch {
		case i == 2 || i == 5:
			_, err := svc.DeclareRest(ctx, doer, ci.ID)
			return err
		case i%3 == 0:
			if _, err := svc.SubmitProof(ctx, doer, ci.ID, proof); err != nil {
				return err
			}
			_, err := svc.Approve(ctx, backer, ci.ID)
			return err
		case i == 4:
			if _, err := svc.SubmitProof(ctx, doer, ci.ID, proof); err != nil {
				return err
			}
			_, err := svc.Reject(ctx, backer, ci.ID, "Belum sesuai target")
			return err
		}
		return nil // missed
	}
	if i%4 == 0 {
		if _, err := svc.SubmitProof(ctx, backer, ci.ID, proof); err != nil {
			return err
		}
		_, err := svc.Approve(ctx, doer, ci.ID)
		return err
	}
	return nil
}

func checkInsOn(ctx context.Context, st *store.Store, pact uuid.UUID, day domain.Date) ([]store.CheckIn, error) {
	return st.ListCheckInsForPact(ctx, store.ListCheckInsForPactParams{PactID: pact, FromDate: day.Time(), ToDate: day.Time()})
}

// Advanced says what Advance did.
type Advanced struct {
	At    time.Time // the instant the clock was moved to
	Moved int       // check-ins the sweep moved (a miss is one)
}

var ErrNothingToAdvance = errors.New("this pact has no open check-in left to run past its deadline")

// Advance is the e2e's test clock (decided with the owner on 2026-10-08: no endpoint). It moves a
// clock to one second after the pact's soonest open submit deadline and runs the real sweep, so
// the miss is printed by the same service code the worker runs. The sweep is not scoped to the
// pact, so it also settles anything else that was due by then; the e2e seeds a pact whose cutoff
// (23:59, no grace) comes before every other seeded deadline, which keeps that to this pact.
func Advance(ctx context.Context, st *store.Store, pactID uuid.UUID) (Advanced, error) {
	rows, err := st.ListCheckInsForPact(ctx, store.ListCheckInsForPactParams{
		PactID: pactID, FromDate: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), ToDate: time.Date(2200, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		return Advanced{}, err
	}
	var next *store.CheckIn
	for i := range rows {
		if rows[i].Status == "open" && (next == nil || rows[i].SubmitDeadline.Before(next.SubmitDeadline)) {
			next = &rows[i]
		}
	}
	if next == nil {
		return Advanced{}, ErrNothingToAdvance
	}
	at := next.SubmitDeadline.Add(time.Second)
	moved, err := service.New(st, domain.NewFakeClock(at)).SweepDeadlines(ctx, 500)
	if err != nil {
		return Advanced{}, fmt.Errorf("sweep: %w", err)
	}
	return Advanced{At: at, Moved: moved}, nil
}
