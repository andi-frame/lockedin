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

// reviewPacts are the three pacts SeedReview builds, each with the doer's proof for today already
// sent. The backer does not commit, so every check-in is the doer's and the backer is its reviewer.
var reviewPacts = []struct {
	title        string
	autoApproved bool
}{
	{title: "Review: approve"},
	{title: "Review: dispute"},
	{title: "Review: override", autoApproved: true},
}

// SeedReview gives one backer a review queue with two submitted proofs (to approve, and to reject
// and then see disputed) and a third that the clock has already auto-approved, inside its override
// window. The review spec signs in as the backer, and as the doer for the dispute. As everywhere
// in the seeds, it goes through the service with a clock (AGENTS.md invariant 3).
func SeedReview(ctx context.Context, st *store.Store, now time.Time, tag string) (Seeded, error) {
	loc, err := time.LoadLocation(seedTimezone)
	if err != nil {
		return Seeded{}, err
	}
	backer, err := seedUser(ctx, st, fmt.Sprintf("review-%s-backer@tepati.test", tag), "Andi (tinjau)")
	if err != nil {
		return Seeded{}, err
	}
	doer, err := seedUser(ctx, st, fmt.Sprintf("review-%s-doer@tepati.test", tag), "Bima (tinjau)")
	if err != nil {
		return Seeded{}, err
	}
	today := localDate(now, loc, 0)
	clock := domain.NewFakeClock(noon(today, loc).Add(-24 * time.Hour))
	svc := service.New(st, clock)

	pacts := make([]store.Pact, 0, len(reviewPacts))
	for _, sc := range reviewPacts {
		terms := seedTerms(backer.ID, today, today.AddDays(13))
		terms.BackerCommits = false
		for id, m := range terms.Members {
			m.Evidence = domain.Evidence{}
			if m.Role == domain.RoleBacker {
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
	proof := service.ProofInput{BodyDoc: []byte(todayProof)}
	for i, sc := range reviewPacts {
		cis, err := checkInsOn(ctx, st, pacts[i].ID, today)
		if err != nil {
			return Seeded{}, err
		}
		for _, ci := range cis {
			if ci.MemberID != doer.ID {
				continue
			}
			submitted, err := svc.SubmitProof(ctx, doer.ID, ci.ID, proof)
			if err != nil {
				return Seeded{}, fmt.Errorf("%s: submit: %w", sc.title, err)
			}
			if sc.autoApproved && submitted.ReviewDeadline != nil {
				// Past the review deadline nobody decided, so the system approves (SPEC §7 step 2).
				// Only this check-in is ticked, with the clock set just past that deadline.
				clock.Set(submitted.ReviewDeadline.Add(time.Minute))
				if _, err := svc.SweepCheckIns(ctx, []uuid.UUID{ci.ID}); err != nil {
					return Seeded{}, fmt.Errorf("%s: auto-approve: %w", sc.title, err)
				}
				clock.Set(now)
			}
		}
	}
	for i := range pacts {
		if pacts[i], err = st.GetPact(ctx, pacts[i].ID); err != nil {
			return Seeded{}, err
		}
	}
	return Seeded{Pact: pacts[0], Pacts: pacts, Backer: backer, Doer: doer}, nil
}
