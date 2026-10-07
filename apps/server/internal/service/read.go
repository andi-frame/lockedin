package service

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/andi-frame/lockedin/apps/server/internal/domain"
	"github.com/andi-frame/lockedin/apps/server/internal/store"
)

// This file holds the read side the HTTP layer needs: membership-filtered views
// (AGENTS.md invariant 8), keyset pagination, and the per-viewer action list.
// Every view of pact data returns ErrNotFound for non-members.

const (
	defaultPageSize = 30
	maxPageSize     = 100
)

func clampLimit(n int) int {
	switch {
	case n <= 0:
		return defaultPageSize
	case n > maxPageSize:
		return maxPageSize
	}
	return n
}

// ---------------------------------------------------------------- pacts

type MemberView struct {
	UserID        uuid.UUID
	DisplayName   string
	Role          domain.Role
	LineColor     string
	Accepted      bool // signed the current terms_hash
	AcceptedAt    *time.Time
	SignatureName *string
	RestDaysUsed  int
}

type PactView struct {
	Pact    store.Pact
	Terms   domain.Terms
	MyRole  domain.Role
	Balance int64
	Members []MemberView // the backer first
	Payout  *store.Payout
}

type PactCursor struct {
	At time.Time
	ID uuid.UUID
}

type memberRow struct {
	PactID uuid.UUID
	View   MemberView
}

func toMemberView(pactHash string, userID uuid.UUID, role, color string, hash *string, at *time.Time, sig *string, rest int32, name string) MemberView {
	return MemberView{
		UserID: userID, DisplayName: name, Role: domain.Role(role), LineColor: color,
		Accepted: hash != nil && *hash == pactHash, AcceptedAt: at, SignatureName: sig, RestDaysUsed: int(rest),
	}
}

// loadPactViews builds views for pacts the caller is already known to belong to,
// with three batched queries instead of three per pact.
func (s *Service) loadPactViews(ctx context.Context, user uuid.UUID, pacts []store.Pact) ([]PactView, error) {
	if len(pacts) == 0 {
		return nil, nil
	}
	ids := make([]uuid.UUID, len(pacts))
	hashes := map[uuid.UUID]string{}
	for i, p := range pacts {
		ids[i], hashes[p.ID] = p.ID, p.TermsHash
	}
	members, err := s.st.ListMembersForPacts(ctx, ids)
	if err != nil {
		return nil, err
	}
	balances, err := s.st.PotBalances(ctx, ids)
	if err != nil {
		return nil, err
	}
	payouts, err := s.st.ListPayoutsForPacts(ctx, ids)
	if err != nil {
		return nil, err
	}

	byPact := map[uuid.UUID][]MemberView{}
	for _, m := range members {
		byPact[m.PactID] = append(byPact[m.PactID], toMemberView(hashes[m.PactID], m.UserID, m.Role, m.LineColor, m.AcceptedTermsHash, m.AcceptedAt, m.SignatureName, m.RestDaysUsed, m.DisplayName))
	}
	balance := map[uuid.UUID]int64{}
	for _, b := range balances {
		balance[b.PactID] = b.Balance
	}
	payout := map[uuid.UUID]*store.Payout{}
	for i := range payouts {
		payout[payouts[i].PactID] = &payouts[i]
	}

	out := make([]PactView, len(pacts))
	for i, p := range pacts {
		terms, err := loadTerms(p)
		if err != nil {
			return nil, err
		}
		v := PactView{Pact: p, Terms: terms, Balance: balance[p.ID], Members: byPact[p.ID], Payout: payout[p.ID]}
		for _, m := range v.Members {
			if m.UserID == user {
				v.MyRole = m.Role
			}
		}
		out[i] = v
	}
	return out, nil
}

// GetPactView is the pact detail for a member. Non-members get ErrNotFound.
func (s *Service) GetPactView(ctx context.Context, user, pactID uuid.UUID) (PactView, error) {
	p, err := s.st.GetPactForMember(ctx, store.GetPactForMemberParams{UserID: user, PactID: pactID})
	if store.IsNoRows(err) {
		return PactView{}, ErrNotFound
	}
	if err != nil {
		return PactView{}, err
	}
	views, err := s.loadPactViews(ctx, user, []store.Pact{p})
	if err != nil {
		return PactView{}, err
	}
	return views[0], nil
}

// ListPactViews returns the caller's pacts, newest first, plus the cursor for the next page.
func (s *Service) ListPactViews(ctx context.Context, user uuid.UUID, after *PactCursor, limit int) ([]PactView, *PactCursor, error) {
	limit = clampLimit(limit)
	params := store.ListPactsForUserPageParams{UserID: user, MaxRows: int32(limit + 1)}
	if after != nil {
		params.BeforeAt, params.BeforeID = &after.At, &after.ID
	}
	rows, err := s.st.ListPactsForUserPage(ctx, params)
	if err != nil {
		return nil, nil, err
	}
	var next *PactCursor
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[limit-1]
		next = &PactCursor{At: last.CreatedAt, ID: last.ID}
	}
	views, err := s.loadPactViews(ctx, user, rows)
	return views, next, err
}

type InviteView struct {
	Pact         store.Pact
	Terms        domain.Terms
	Inviter      MemberView
	DoerSlotOpen bool
}

// InvitePreview is what an invitee may see before joining: the terms and who invited them.
func (s *Service) InvitePreview(ctx context.Context, token string) (InviteView, error) {
	p, err := s.PreviewInvite(ctx, token)
	if err != nil {
		return InviteView{}, err
	}
	terms, err := loadTerms(p)
	if err != nil {
		return InviteView{}, err
	}
	members, err := s.st.ListPactMembers(ctx, p.ID)
	if err != nil {
		return InviteView{}, err
	}
	v := InviteView{Pact: p, Terms: terms, DoerSlotOpen: doerKey(terms, p.BackerID) == uuid.Nil}
	for _, m := range members {
		if m.UserID == p.BackerID {
			v.Inviter = toMemberView(p.TermsHash, m.UserID, m.Role, m.LineColor, m.AcceptedTermsHash, m.AcceptedAt, m.SignatureName, m.RestDaysUsed, m.DisplayName)
		}
	}
	return v, nil
}

// ---------------------------------------------------------------- check-ins

// Action is something the viewer may do to a check-in right now.
type Action string

const (
	ActionSubmit         Action = "submit"
	ActionEditProof      Action = "edit_proof"
	ActionDeclareRest    Action = "declare_rest"
	ActionApprove        Action = "approve"
	ActionReject         Action = "reject"
	ActionOverride       Action = "override"
	ActionDispute        Action = "dispute"
	ActionResolveDispute Action = "resolve_dispute"
)

type DecisionView struct {
	store.ListDecisionsRow
	IsPower bool // the backer's extra powers: override, uphold, dismiss
}

type CheckInView struct {
	CheckIn            store.CheckIn
	PactTitle          string
	Member, Reviewer   MemberView
	Proof              *store.Proof
	Versions           []store.Proof // oldest first
	Attachments        []store.Attachment
	Decisions          []DecisionView // oldest first
	Actions            []Action
	OverridesRemaining *int // the backer's remaining overrides; nil for the doer
}

// A placeholder that satisfies the 10-character reason rule when dry-running transitions.
const dryRunReason = "dry run only"

// allowedActions dry-runs the real state machine for each user action, so the client
// never re-derives the rules and cannot drift from them.
func (s *Service) allowedActions(pact store.Pact, terms domain.Terms, ci store.CheckIn, restUsed int, viewer uuid.UUID) []Action {
	if pact.Status != "active" {
		return nil
	}
	tc := domain.TransitionCtx{
		Terms: terms, BackerID: pact.BackerID, OverridesUsed: int(pact.OverridesUsed),
		RestDaysUsed: restUsed, EvidenceOK: true,
	}
	d, now := toDomain(ci, terms), s.clock.Now()
	can := func(kind domain.EventKind) bool {
		_, _, err := domain.Transition(d, domain.Event{Kind: kind, ActorID: viewer, Reason: dryRunReason}, now, tc)
		return err == nil
	}

	var out []Action
	if can(domain.EventSubmit) {
		if ci.Status == "open" {
			out = append(out, ActionSubmit)
		} else {
			out = append(out, ActionEditProof)
		}
	}
	for _, a := range []struct {
		kind   domain.EventKind
		action Action
	}{
		{domain.EventDeclareRest, ActionDeclareRest},
		{domain.EventApprove, ActionApprove},
		{domain.EventReject, ActionReject},
		{domain.EventOverride, ActionOverride},
		{domain.EventDispute, ActionDispute},
	} {
		if can(a.kind) {
			out = append(out, a.action)
		}
	}
	if can(domain.EventUphold) || can(domain.EventDismiss) {
		out = append(out, ActionResolveDispute)
	}
	return out
}

func isPower(action string) bool {
	return action == string(domain.ActionOverride) || action == string(domain.ActionUphold) || action == string(domain.ActionDismiss)
}

// GetCheckInView is the check-in detail for a pact member.
func (s *Service) GetCheckInView(ctx context.Context, user, id uuid.UUID) (CheckInView, error) {
	ci, err := s.st.GetCheckInForMember(ctx, store.GetCheckInForMemberParams{UserID: user, ID: id})
	if store.IsNoRows(err) {
		return CheckInView{}, ErrNotFound
	}
	if err != nil {
		return CheckInView{}, err
	}
	pact, err := s.st.GetPact(ctx, ci.PactID)
	if err != nil {
		return CheckInView{}, err
	}
	terms, err := loadTerms(pact)
	if err != nil {
		return CheckInView{}, err
	}
	members, err := s.st.ListPactMembers(ctx, pact.ID)
	if err != nil {
		return CheckInView{}, err
	}

	v := CheckInView{CheckIn: ci, PactTitle: pact.Title}
	restUsed := 0
	for _, m := range members {
		mv := toMemberView(pact.TermsHash, m.UserID, m.Role, m.LineColor, m.AcceptedTermsHash, m.AcceptedAt, m.SignatureName, m.RestDaysUsed, m.DisplayName)
		if m.UserID == ci.MemberID {
			v.Member, restUsed = mv, int(m.RestDaysUsed)
		}
		if m.UserID == ci.ReviewerID {
			v.Reviewer = mv
		}
	}

	proofs, err := s.st.ListProofs(ctx, id) // newest first
	if err != nil {
		return CheckInView{}, err
	}
	v.Versions = slices.Clone(proofs)
	slices.Reverse(v.Versions)
	if len(proofs) > 0 {
		v.Proof = &proofs[0]
		if v.Attachments, err = s.st.ListAttachmentsForProof(ctx, &proofs[0].ID); err != nil {
			return CheckInView{}, err
		}
	}
	decisions, err := s.st.ListDecisions(ctx, id)
	if err != nil {
		return CheckInView{}, err
	}
	for _, d := range decisions {
		v.Decisions = append(v.Decisions, DecisionView{ListDecisionsRow: d, IsPower: isPower(d.Action)})
	}

	v.Actions = s.allowedActions(pact, terms, ci, restUsed, user)
	if user == pact.BackerID {
		left := max(0, terms.MaxOverrides-int(pact.OverridesUsed))
		v.OverridesRemaining = &left
	}
	return v, nil
}

// ListPactCheckIns returns both members' check-ins for the calendar. from and to default to
// the pact's own dates.
func (s *Service) ListPactCheckIns(ctx context.Context, user, pactID uuid.UUID, from, to *time.Time) ([]store.CheckIn, error) {
	p, err := s.st.GetPactForMember(ctx, store.GetPactForMemberParams{UserID: user, PactID: pactID})
	if store.IsNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	params := store.ListCheckInsForPactParams{PactID: pactID, FromDate: p.StartsOn, ToDate: p.EndsOn}
	if from != nil {
		params.FromDate = *from
	}
	if to != nil {
		params.ToDate = *to
	}
	return s.st.ListCheckInsForPact(ctx, params)
}

// ---------------------------------------------------------------- ledger

type LedgerView struct {
	Lines   []store.ListLedgerPageRow // newest first
	Next    *int64                    // pass as `before` for the next page
	Balance int64
}

// LedgerPage is one page of the passbook. balance_after is computed over the whole
// ledger, so it is the same whichever page a line lands on.
func (s *Service) LedgerPage(ctx context.Context, user, pactID uuid.UUID, before *int64, limit int) (LedgerView, error) {
	if _, err := s.st.GetPactForMember(ctx, store.GetPactForMemberParams{UserID: user, PactID: pactID}); store.IsNoRows(err) {
		return LedgerView{}, ErrNotFound
	} else if err != nil {
		return LedgerView{}, err
	}
	limit = clampLimit(limit)
	rows, err := s.st.ListLedgerPage(ctx, store.ListLedgerPageParams{PactID: pactID, BeforeID: before, MaxRows: int32(limit + 1)})
	if err != nil {
		return LedgerView{}, err
	}
	v := LedgerView{}
	if len(rows) > limit {
		rows = rows[:limit]
		next := rows[limit-1].ID
		v.Next = &next
	}
	v.Lines = rows
	if v.Balance, err = s.st.PotBalance(ctx, pactID); err != nil {
		return LedgerView{}, err
	}
	return v, nil
}

// ---------------------------------------------------------------- review queue

type ReviewCursor struct {
	Deadline time.Time
	ID       uuid.UUID
}

type ReviewItem struct {
	store.ListReviewQueuePageRow
	Member MemberView // whose submission it is
}

// ReviewQueue lists submissions waiting for the caller, soonest review deadline first.
func (s *Service) ReviewQueue(ctx context.Context, user uuid.UUID, after *ReviewCursor, limit int) ([]ReviewItem, *ReviewCursor, error) {
	limit = clampLimit(limit)
	params := store.ListReviewQueuePageParams{ReviewerID: user, MaxRows: int32(limit + 1)}
	if after != nil {
		params.AfterDeadline, params.AfterID = &after.Deadline, &after.ID
	}
	rows, err := s.st.ListReviewQueuePage(ctx, params)
	if err != nil {
		return nil, nil, err
	}
	var next *ReviewCursor
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[limit-1]
		next = &ReviewCursor{Deadline: *last.CheckIn.ReviewDeadline, ID: last.CheckIn.ID}
	}
	if len(rows) == 0 {
		return nil, nil, nil
	}

	pactIDs := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		if !slices.Contains(pactIDs, r.CheckIn.PactID) {
			pactIDs = append(pactIDs, r.CheckIn.PactID)
		}
	}
	members, err := s.memberIndex(ctx, pactIDs)
	if err != nil {
		return nil, nil, err
	}
	items := make([]ReviewItem, len(rows))
	for i, r := range rows {
		items[i] = ReviewItem{ListReviewQueuePageRow: r, Member: members[memberKey{r.CheckIn.PactID, r.CheckIn.MemberID}]}
	}
	return items, next, nil
}

type memberKey struct{ pact, user uuid.UUID }

func (s *Service) memberIndex(ctx context.Context, pactIDs []uuid.UUID) (map[memberKey]MemberView, error) {
	rows, err := s.st.ListMembersForPacts(ctx, pactIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[memberKey]MemberView, len(rows))
	for _, m := range rows {
		out[memberKey{m.PactID, m.UserID}] = toMemberView("", m.UserID, m.Role, m.LineColor, m.AcceptedTermsHash, m.AcceptedAt, m.SignatureName, m.RestDaysUsed, m.DisplayName)
	}
	return out, nil
}

// ---------------------------------------------------------------- today

type TodayCheckIn struct {
	store.ListTodayCheckInsRow
	Member    MemberView
	WordCount *int32 // of the latest proof; nil when nothing was submitted
}

type TodayPact struct {
	Pact         store.Pact
	MyRole       domain.Role
	Partner      MemberView
	Balance      int64
	NextDeadline *time.Time // my nearest open submit deadline in this pact
	Recent       []store.ListLedgerPageRow
}

type TodayView struct {
	ServerTime  time.Time
	CheckIns    []TodayCheckIn
	ReviewCount int64
	Pacts       []TodayPact // nearest deadline first
}

// Today is everything the Today screen needs in one call.
func (s *Service) Today(ctx context.Context, user uuid.UUID) (TodayView, error) {
	now := s.clock.Now()
	v := TodayView{ServerTime: now}

	rows, err := s.st.ListTodayCheckIns(ctx, store.ListTodayCheckInsParams{MemberID: user, Now: now})
	if err != nil {
		return v, err
	}
	if v.ReviewCount, err = s.st.CountReviewQueue(ctx, user); err != nil {
		return v, err
	}
	all, err := s.st.ListPactsForUser(ctx, user)
	if err != nil {
		return v, err
	}
	var live []store.Pact
	for _, p := range all {
		if p.Status == "active" || p.Status == "settling" {
			live = append(live, p)
		}
	}
	views, err := s.loadPactViews(ctx, user, live)
	if err != nil {
		return v, err
	}

	ids := make([]uuid.UUID, len(views))
	for i, pv := range views {
		ids[i] = pv.Pact.ID
	}
	deadlines := map[uuid.UUID]time.Time{}
	if len(ids) > 0 {
		next, err := s.st.NextDeadlines(ctx, store.NextDeadlinesParams{MemberID: user, PactIds: ids})
		if err != nil {
			return v, err
		}
		for _, d := range next {
			deadlines[d.PactID] = d.NextDeadline
		}
	}

	byMember := map[memberKey]MemberView{}
	for _, pv := range views {
		for _, m := range pv.Members {
			byMember[memberKey{pv.Pact.ID, m.UserID}] = m
		}
	}
	for _, r := range rows {
		tc := TodayCheckIn{ListTodayCheckInsRow: r, Member: byMember[memberKey{r.CheckIn.PactID, r.CheckIn.MemberID}]}
		if r.HasProof {
			w := r.WordCount
			tc.WordCount = &w
		}
		v.CheckIns = append(v.CheckIns, tc)
	}

	for _, pv := range views {
		tp := TodayPact{Pact: pv.Pact, MyRole: pv.MyRole, Balance: pv.Balance}
		for _, m := range pv.Members {
			if m.UserID != user {
				tp.Partner = m
			}
		}
		if d, ok := deadlines[pv.Pact.ID]; ok {
			tp.NextDeadline = &d
		}
		if tp.Recent, err = s.st.ListLedgerPage(ctx, store.ListLedgerPageParams{PactID: pv.Pact.ID, MaxRows: 3}); err != nil {
			return v, err
		}
		v.Pacts = append(v.Pacts, tp)
	}
	slices.SortStableFunc(v.Pacts, func(a, b TodayPact) int {
		switch {
		case a.NextDeadline == nil && b.NextDeadline == nil:
			return 0
		case a.NextDeadline == nil:
			return 1
		case b.NextDeadline == nil:
			return -1
		}
		return a.NextDeadline.Compare(*b.NextDeadline)
	})
	return v, nil
}

// ---------------------------------------------------------------- notifications and users

// ListNotifications is the caller's inbox, newest first, with the total unread count.
func (s *Service) ListNotifications(ctx context.Context, user uuid.UUID, before *int64, limit int, unreadOnly bool) ([]store.Notification, *int64, int64, error) {
	limit = clampLimit(limit)
	rows, err := s.st.ListNotifications(ctx, store.ListNotificationsParams{UserID: user, BeforeID: before, UnreadOnly: unreadOnly, MaxRows: int32(limit + 1)})
	if err != nil {
		return nil, nil, 0, err
	}
	var next *int64
	if len(rows) > limit {
		rows = rows[:limit]
		id := rows[limit-1].ID
		next = &id
	}
	unread, err := s.st.CountUnreadNotifications(ctx, user)
	return rows, next, unread, err
}

// MarkNotificationsRead ignores ids that are already read or belong to someone else.
func (s *Service) MarkNotificationsRead(ctx context.Context, user uuid.UUID, ids []int64) error {
	return s.st.MarkNotificationsRead(ctx, store.MarkNotificationsReadParams{UserID: user, Ids: ids})
}

// Me returns the signed-in user's account.
func (s *Service) Me(ctx context.Context, id uuid.UUID) (store.User, error) {
	u, err := s.st.GetUser(ctx, id)
	if store.IsNoRows(err) {
		return store.User{}, ErrNotFound
	}
	return u, err
}
