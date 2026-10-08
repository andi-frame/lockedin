package ctl

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/andi-frame/lockedin/apps/server/internal/domain"
	"github.com/andi-frame/lockedin/apps/server/internal/service"
	"github.com/andi-frame/lockedin/apps/server/internal/store"
)

// settlementTitles are the pacts SeedSettlement builds. They are identical; the settlement spec
// takes the first through "marked paid, then confirmed" and the second through "the doer confirms
// alone" (SPEC §3: the doer's confirmation is sufficient).
var settlementTitles = []string{"Settlement: paid first", "Settlement: doer only"}

const settlementDays = 5

// SeedSettlement gives one backer and one doer pacts whose five days are over: the doer sent proof
// and was approved on days 0, 2 and 4 and missed days 1 and 3, so the pot of 1000 paid two
// penalties of 50 and 900 coins are owed. The pacts end five days before `now`, and the clock is
// walked past every window, so each check-in is final and the real ClosePacts moves the pact to
// `settling` with its payout line. Everything goes through the service with a clock (AGENTS.md
// invariant 3); the clock never goes past `now`, so nothing it settles is not already due.
func SeedSettlement(ctx context.Context, st *store.Store, now time.Time, tag string) (Seeded, error) {
	loc, err := time.LoadLocation(seedTimezone)
	if err != nil {
		return Seeded{}, err
	}
	backer, err := seedUser(ctx, st, fmt.Sprintf("settlement-%s-backer@tepati.test", tag), "Andi (bayar)")
	if err != nil {
		return Seeded{}, err
	}
	doer, err := seedUser(ctx, st, fmt.Sprintf("settlement-%s-doer@tepati.test", tag), "Bima (bayar)")
	if err != nil {
		return Seeded{}, err
	}
	today := localDate(now, loc, 0)
	end := today.AddDays(-5)
	start := end.AddDays(-(settlementDays - 1))
	clock := domain.NewFakeClock(noon(start, loc).Add(-24 * time.Hour))
	svc := service.New(st, clock)
	proof := service.ProofInput{BodyDoc: []byte(todayProof)}

	pacts := make([]store.Pact, 0, len(settlementTitles))
	for _, title := range settlementTitles {
		clock.Set(noon(start, loc).Add(-24 * time.Hour))
		terms := seedTerms(backer.ID, start, end)
		terms.GraceMinutes = 0
		terms.BackerCommits = false
		for id, m := range terms.Members {
			m.Evidence = domain.Evidence{}
			if m.Role == domain.RoleBacker {
				m.Schedule = nil
			}
			terms.Members[id] = m
		}
		p, err := svc.CreateDraft(ctx, backer.ID, service.DraftInput{Title: title, Terms: terms})
		if err != nil {
			return Seeded{}, fmt.Errorf("%s: draft: %w", title, err)
		}
		token, err := svc.Propose(ctx, backer.ID, p.ID, nil)
		if err != nil {
			return Seeded{}, fmt.Errorf("%s: propose: %w", title, err)
		}
		if p, err = svc.JoinByInvite(ctx, doer.ID, token); err != nil {
			return Seeded{}, fmt.Errorf("%s: join: %w", title, err)
		}
		for _, signer := range []store.User{backer, doer} {
			if _, err := svc.Accept(ctx, signer.ID, p.ID, p.TermsHash, signer.DisplayName); err != nil {
				return Seeded{}, fmt.Errorf("%s: accept (%s): %w", title, signer.DisplayName, err)
			}
		}
		clock.Set(noon(start, loc))
		if _, err := svc.ActivateDuePacts(ctx); err != nil {
			return Seeded{}, fmt.Errorf("%s: activate: %w", title, err)
		}

		all, err := st.ListCheckInsForPact(ctx, store.ListCheckInsForPactParams{PactID: p.ID, FromDate: start.Time(), ToDate: end.Time()})
		if err != nil {
			return Seeded{}, err
		}
		ids := make([]uuid.UUID, 0, len(all))
		for _, ci := range all {
			ids = append(ids, ci.ID)
		}
		for i := 0; i < settlementDays; i++ {
			day := start.AddDays(i)
			clock.Set(noon(day, loc).Add(-4 * time.Hour))
			cis, err := checkInsOn(ctx, st, p.ID, day)
			if err != nil {
				return Seeded{}, err
			}
			for _, ci := range cis {
				if i%2 != 0 {
					continue // missed: nobody sends anything
				}
				if _, err := svc.SubmitProof(ctx, doer.ID, ci.ID, proof); err != nil {
					return Seeded{}, fmt.Errorf("%s: day %d submit: %w", title, i, err)
				}
				if _, err := svc.Approve(ctx, backer.ID, ci.ID); err != nil {
					return Seeded{}, fmt.Errorf("%s: day %d approve: %w", title, i, err)
				}
			}
			clock.Set(noon(day, loc))
			if _, err := svc.SweepCheckIns(ctx, ids); err != nil {
				return Seeded{}, fmt.Errorf("%s: sweep after day %d: %w", title, i, err)
			}
		}
		// Three days after the last one every window has closed, so the pact can settle.
		clock.Set(noon(end.AddDays(3), loc))
		if _, err := svc.SweepCheckIns(ctx, ids); err != nil {
			return Seeded{}, fmt.Errorf("%s: final sweep: %w", title, err)
		}
		pacts = append(pacts, p)
	}
	if _, err := svc.ClosePacts(ctx, 50); err != nil {
		return Seeded{}, fmt.Errorf("close: %w", err)
	}
	for i := range pacts {
		if pacts[i], err = st.GetPact(ctx, pacts[i].ID); err != nil {
			return Seeded{}, err
		}
	}
	return Seeded{Pact: pacts[0], Pacts: pacts, Backer: backer, Doer: doer}, nil
}
