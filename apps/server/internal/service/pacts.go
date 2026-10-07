package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/andi-frame/lockedin/apps/server/internal/domain"
	"github.com/andi-frame/lockedin/apps/server/internal/store"
)

const inviteTTL = 7 * 24 * time.Hour

// DraftInput describes a new or edited pact. In the MVP the creator is always the
// backer (SPEC §2). Until the doer joins, the doer's terms are keyed by uuid.Nil.
type DraftInput struct {
	Title       string
	Description *string
	Terms       domain.Terms
}

// CreateDraft stores a draft pact with the creator as backer.
func (s *Service) CreateDraft(ctx context.Context, creator uuid.UUID, in DraftInput) (store.Pact, error) {
	if err := checkDraftShape(in.Terms, creator, uuid.Nil); err != nil {
		return store.Pact{}, err
	}
	raw, hash, err := encodeTerms(in.Terms)
	if err != nil {
		return store.Pact{}, err
	}
	var pact store.Pact
	err = s.st.WithTx(ctx, func(q *store.Queries) error {
		open, err := q.CountOpenPactsForUser(ctx, creator)
		if err != nil {
			return err
		}
		if open >= maxOpenPacts {
			return ErrPactLimit
		}
		pact, err = q.CreatePact(ctx, store.CreatePactParams{
			ID: newID(), Title: strings.TrimSpace(in.Title), Description: in.Description,
			CreatedBy: creator, BackerID: creator, Terms: raw, TermsHash: hash,
			Timezone: in.Terms.Timezone, StartsOn: in.Terms.StartsOn.Time(), EndsOn: in.Terms.EndsOn.Time(),
		})
		if err != nil {
			return err
		}
		return q.AddPactMember(ctx, store.AddPactMemberParams{
			PactID: pact.ID, UserID: creator, Role: string(domain.RoleBacker), LineColor: lineColorBacker,
		})
	})
	return pact, err
}

// UpdateTerms edits a draft or proposed pact. Any edit clears both signatures (SPEC §3).
func (s *Service) UpdateTerms(ctx context.Context, actor, pactID uuid.UUID, in DraftInput) (store.Pact, error) {
	var pact store.Pact
	err := s.st.WithTx(ctx, func(q *store.Queries) error {
		p, err := s.memberPactForUpdate(ctx, q, actor, pactID)
		if err != nil {
			return err
		}
		if p.Status != "draft" && p.Status != "proposed" {
			return ErrPactState
		}
		current, err := loadTerms(p)
		if err != nil {
			return err
		}
		if err := checkDraftShape(in.Terms, p.BackerID, doerKey(current, p.BackerID)); err != nil {
			return err
		}
		raw, hash, err := encodeTerms(in.Terms)
		if err != nil {
			return err
		}
		if _, err := q.UpdatePactTerms(ctx, store.UpdatePactTermsParams{
			ID: pactID, Title: strings.TrimSpace(in.Title), Description: in.Description,
			Terms: raw, TermsHash: hash, Timezone: in.Terms.Timezone,
			StartsOn: in.Terms.StartsOn.Time(), EndsOn: in.Terms.EndsOn.Time(),
		}); err != nil {
			return err
		}
		if err := q.ResetAcceptances(ctx, pactID); err != nil {
			return err
		}
		pact, err = q.GetPact(ctx, pactID)
		if err != nil {
			return err
		}
		return s.notifyOtherMember(ctx, q, pact, actor, "terms_changed")
	})
	return pact, err
}

// Propose moves a draft to proposed and returns a one-time invite token.
// Only the token's hash is stored.
func (s *Service) Propose(ctx context.Context, actor, pactID uuid.UUID, email *string) (string, error) {
	token := randomToken()
	err := s.st.WithTx(ctx, func(q *store.Queries) error {
		p, err := s.memberPactForUpdate(ctx, q, actor, pactID)
		if err != nil {
			return err
		}
		if p.BackerID != actor {
			return ErrNotBacker
		}
		if p.Status == "draft" {
			if _, err := q.SetPactStatus(ctx, store.SetPactStatusParams{ID: pactID, FromStatus: "draft", ToStatus: "proposed"}); err != nil {
				return err
			}
		} else if p.Status != "proposed" {
			return ErrPactState
		}
		if err := q.CreateInvite(ctx, store.CreateInviteParams{
			TokenHash: hashToken(token), PactID: pactID, Email: email, ExpiresAt: s.clock.Now().Add(inviteTTL),
		}); err != nil {
			return err
		}
		if email == nil {
			return nil // a link-only invite: the backer shares it by hand
		}
		return enqueueInviteMail(ctx, q, InviteMail{PactID: pactID, Email: *email, Token: token})
	})
	if err != nil {
		return "", err
	}
	return token, nil
}

// PreviewInvite returns the pact behind a valid invite token, for the invitee to read the terms.
func (s *Service) PreviewInvite(ctx context.Context, token string) (store.Pact, error) {
	inv, err := s.st.GetInvite(ctx, hashToken(token))
	if store.IsNoRows(err) || (err == nil && !inviteUsable(inv, s.clock.Now())) {
		return store.Pact{}, ErrInviteInvalid
	}
	if err != nil {
		return store.Pact{}, err
	}
	return s.st.GetPact(ctx, inv.PactID)
}

// JoinByInvite makes the user the pact's doer. The doer slot in the terms moves from
// uuid.Nil to the user's id, which changes the hash and clears any signature.
func (s *Service) JoinByInvite(ctx context.Context, user uuid.UUID, token string) (store.Pact, error) {
	var pact store.Pact
	err := s.st.WithTx(ctx, func(q *store.Queries) error {
		inv, err := q.GetInvite(ctx, hashToken(token))
		if store.IsNoRows(err) || (err == nil && !inviteUsable(inv, s.clock.Now())) {
			return ErrInviteInvalid
		}
		if err != nil {
			return err
		}
		p, err := q.GetPactForUpdate(ctx, inv.PactID)
		if err != nil {
			return err
		}
		if p.Status != "proposed" {
			return ErrPactState
		}
		if p.BackerID == user {
			return ErrInviteInvalid // the backer cannot join their own pact as doer
		}
		terms, err := loadTerms(p)
		if err != nil {
			return err
		}
		if doerKey(terms, p.BackerID) != uuid.Nil {
			return ErrInviteInvalid // a doer already joined
		}
		open, err := q.CountOpenPactsForUser(ctx, user)
		if err != nil {
			return err
		}
		if open >= maxOpenPacts {
			return ErrPactLimit
		}
		terms.Members[user] = terms.Members[uuid.Nil]
		delete(terms.Members, uuid.Nil)
		raw, hash, err := encodeTerms(terms)
		if err != nil {
			return err
		}
		if _, err := q.UpdatePactTerms(ctx, store.UpdatePactTermsParams{
			ID: p.ID, Title: p.Title, Description: p.Description, Terms: raw, TermsHash: hash,
			Timezone: p.Timezone, StartsOn: p.StartsOn, EndsOn: p.EndsOn,
		}); err != nil {
			return err
		}
		if err := q.AddPactMember(ctx, store.AddPactMemberParams{
			PactID: p.ID, UserID: user, Role: string(domain.RoleDoer), LineColor: lineColorDoer,
		}); err != nil {
			return err
		}
		if err := q.ResetAcceptances(ctx, p.ID); err != nil {
			return err
		}
		if n, err := q.UseInvite(ctx, inv.TokenHash); err != nil || n == 0 {
			return errors.Join(ErrInviteInvalid, err)
		}
		pact, err = q.GetPact(ctx, p.ID)
		if err != nil {
			return err
		}
		return s.notifyOtherMember(ctx, q, pact, user, "member_joined")
	})
	return pact, err
}

// Accept signs the current terms. When both members signed the same hash, the pact
// is scheduled: the pot is funded and every check-in row is generated (SPEC §3).
func (s *Service) Accept(ctx context.Context, user, pactID uuid.UUID, termsHash, signature string) (store.Pact, error) {
	var pact store.Pact
	err := s.st.WithTx(ctx, func(q *store.Queries) error {
		p, err := s.memberPactForUpdate(ctx, q, user, pactID)
		if err != nil {
			return err
		}
		if p.Status != "proposed" {
			return ErrPactState
		}
		if termsHash != p.TermsHash {
			return ErrTermsMismatch
		}
		terms, err := loadTerms(p)
		if err != nil {
			return err
		}
		if _, waiting := terms.Members[uuid.Nil]; waiting {
			return ErrMemberMissing
		}
		u, err := q.GetUser(ctx, user)
		if err != nil {
			return err
		}
		if !strings.EqualFold(strings.TrimSpace(signature), strings.TrimSpace(u.DisplayName)) {
			return ErrSignatureMismatch
		}
		if _, err := q.AcceptTerms(ctx, store.AcceptTermsParams{
			PactID: pactID, UserID: user, TermsHash: &termsHash, SignatureName: &u.DisplayName,
		}); err != nil {
			return err
		}
		signed, err := q.CountAcceptances(ctx, store.CountAcceptancesParams{PactID: pactID, TermsHash: &termsHash})
		if err != nil {
			return err
		}
		if signed == 2 {
			if err := s.schedule(ctx, q, p, terms); err != nil {
				return err
			}
		} else if err := s.notifyOtherMember(ctx, q, p, user, "terms_signed"); err != nil {
			return err
		}
		pact, err = q.GetPact(ctx, pactID)
		return err
	})
	return pact, err
}

func (s *Service) schedule(ctx context.Context, q *store.Queries, p store.Pact, terms domain.Terms) error {
	if n, err := q.SetPactStatus(ctx, store.SetPactStatusParams{ID: p.ID, FromStatus: "proposed", ToStatus: "scheduled"}); err != nil || n != 1 {
		return errors.Join(ErrPactState, err)
	}
	if _, err := q.InsertLedgerEntry(ctx, store.InsertLedgerEntryParams{
		PactID: p.ID, Kind: string(domain.LedgerPotInitial), Amount: terms.InitialPot,
		IdempotencyKey: domain.InitialPotKey(p.ID),
	}); err != nil && !store.IsNoRows(err) {
		return fmt.Errorf("fund pot: %w", err)
	}
	var rows []store.InsertCheckInsParams
	for member := range terms.Members {
		reviewer, err := terms.ReviewerOf(member)
		if err != nil {
			return err
		}
		for _, d := range terms.ScheduledDates(member) {
			cutoff, submit, err := terms.CheckInDeadlines(d)
			if err != nil {
				return err
			}
			rows = append(rows, store.InsertCheckInsParams{
				ID: newID(), PactID: p.ID, MemberID: member, ReviewerID: reviewer,
				LocalDate: d.Time(), CutoffAt: cutoff, SubmitDeadline: submit,
			})
		}
	}
	if _, err := q.InsertCheckIns(ctx, rows); err != nil {
		return fmt.Errorf("generate check-ins: %w", err)
	}
	for member := range terms.Members {
		if err := enqueueNotification(ctx, q, Notification{UserID: member, Kind: "pact_scheduled", PactID: p.ID}); err != nil {
			return err
		}
	}
	return nil
}

// ActivateDuePacts moves scheduled pacts whose start date arrived (in their own
// timezone) to active. Safe to run repeatedly.
func (s *Service) ActivateDuePacts(ctx context.Context) ([]uuid.UUID, error) {
	return s.st.ActivateDuePacts(ctx, s.clock.Now())
}

// GetPact returns a pact only to its members.
func (s *Service) GetPact(ctx context.Context, user, pactID uuid.UUID) (store.Pact, error) {
	p, err := s.st.GetPactForMember(ctx, store.GetPactForMemberParams{UserID: user, PactID: pactID})
	if store.IsNoRows(err) {
		return store.Pact{}, ErrNotFound
	}
	return p, err
}

// memberPactForUpdate locks the pact row after checking membership.
func (s *Service) memberPactForUpdate(ctx context.Context, q *store.Queries, user, pactID uuid.UUID) (store.Pact, error) {
	if _, err := q.GetPactMember(ctx, store.GetPactMemberParams{PactID: pactID, UserID: user}); store.IsNoRows(err) {
		return store.Pact{}, ErrNotFound
	} else if err != nil {
		return store.Pact{}, err
	}
	return q.GetPactForUpdate(ctx, pactID)
}

func (s *Service) notifyOtherMember(ctx context.Context, q *store.Queries, p store.Pact, actor uuid.UUID, kind string) error {
	members, err := q.ListPactMembers(ctx, p.ID)
	if err != nil {
		return err
	}
	for _, m := range members {
		if m.UserID != actor {
			if err := enqueueNotification(ctx, q, Notification{UserID: m.UserID, Kind: kind, PactID: p.ID}); err != nil {
				return err
			}
		}
	}
	return nil
}

// checkDraftShape requires exactly the backer plus one doer slot (the joined doer,
// or uuid.Nil before anyone joined), then validates the rules themselves.
func checkDraftShape(t domain.Terms, backer, doer uuid.UUID) error {
	if b, ok := t.Members[backer]; !ok || b.Role != domain.RoleBacker {
		return ErrTermsShape
	}
	if d, ok := t.Members[doer]; !ok || d.Role != domain.RoleDoer || len(t.Members) != 2 {
		return ErrTermsShape
	}
	return t.Validate()
}

func doerKey(t domain.Terms, backer uuid.UUID) uuid.UUID {
	for id := range t.Members {
		if id != backer {
			return id
		}
	}
	return uuid.Nil
}

func encodeTerms(t domain.Terms) ([]byte, string, error) {
	raw, err := t.MarshalCanonical()
	if err != nil {
		return nil, "", err
	}
	hash, err := t.Hash()
	return raw, hash, err
}

func inviteUsable(inv store.PactInvite, now time.Time) bool {
	return inv.UsedAt == nil && now.Before(inv.ExpiresAt)
}

func randomToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
