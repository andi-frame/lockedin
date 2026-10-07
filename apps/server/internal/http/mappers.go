package http

import (
	"encoding/json"
	"fmt"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/andi-frame/lockedin/apps/server/internal/domain"
	"github.com/andi-frame/lockedin/apps/server/internal/http/api"
	"github.com/andi-frame/lockedin/apps/server/internal/service"
	"github.com/andi-frame/lockedin/apps/server/internal/store"
)

// Conversions between service views and the generated API types. Handlers hold no
// rules: they translate, call one service function, and translate back.

func ptr[T any](v T) *T { return &v }

func apiDate(t time.Time) openapi_types.Date { return openapi_types.Date{Time: t} }

func apiDatePtr(t time.Time, valid bool) *openapi_types.Date {
	if !valid {
		return nil
	}
	d := apiDate(t)
	return &d
}

func apiUser(u store.User) api.User {
	return api.User{
		Id: u.ID, Email: openapi_types.Email(u.Email), DisplayName: u.DisplayName,
		Locale: api.UserLocale(u.Locale), Timezone: u.Timezone, CreatedAt: u.CreatedAt,
	}
}

func apiRef(m service.MemberView) api.MemberRef {
	return api.MemberRef{UserId: m.UserID, DisplayName: m.DisplayName, LineColor: m.LineColor}
}

func apiMember(m service.MemberView) api.PactMember {
	return api.PactMember{
		UserId: m.UserID, DisplayName: m.DisplayName, Role: api.Role(m.Role), LineColor: m.LineColor,
		Accepted: m.Accepted, AcceptedAt: m.AcceptedAt, SignatureName: m.SignatureName, RestDaysUsed: m.RestDaysUsed,
	}
}

// apiTerms and termsFromAPI go through JSON: the contract mirrors domain.Terms field for
// field, so one encoding is the single source of truth for both directions.
func apiTerms(t domain.Terms) (api.Terms, error) {
	var out api.Terms
	raw, err := t.MarshalCanonical()
	if err != nil {
		return out, err
	}
	return out, json.Unmarshal(raw, &out)
}

func termsFromAPI(t api.Terms) (domain.Terms, error) {
	raw, err := json.Marshal(t)
	if err != nil {
		return domain.Terms{}, err
	}
	return domain.ParseTerms(raw)
}

func apiPayout(p store.Payout) api.Payout {
	return api.Payout{Amount: p.Amount, MarkedPaidAt: p.MarkedPaidAt, MarkedPaidNote: p.MarkedPaidNote, ConfirmedAt: p.ConfirmedAt}
}

func apiPact(v service.PactView) (api.Pact, error) {
	terms, err := apiTerms(v.Terms)
	if err != nil {
		return api.Pact{}, fmt.Errorf("pact %s terms: %w", v.Pact.ID, err)
	}
	p := v.Pact
	out := api.Pact{
		Id: p.ID, Title: p.Title, Description: p.Description, Status: api.PactStatus(p.Status),
		BackerId: p.BackerID, MyRole: api.Role(v.MyRole), Terms: terms, TermsVersion: int(p.TermsVersion),
		TermsHash: p.TermsHash, Timezone: p.Timezone, StartsOn: apiDate(p.StartsOn), EndsOn: apiDate(p.EndsOn),
		OverridesUsed: int(p.OverridesUsed), Balance: v.Balance, Members: make([]api.PactMember, len(v.Members)),
		ScheduledAt: p.ScheduledAt, SettledAt: p.SettledAt, CompletedAt: p.CompletedAt,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
	for i, m := range v.Members {
		out.Members[i] = apiMember(m)
	}
	if v.Payout != nil {
		out.Payout = ptr(apiPayout(*v.Payout))
	}
	return out, nil
}

func apiCheckIn(c store.CheckIn) api.CheckIn {
	return api.CheckIn{
		Id: c.ID, PactId: c.PactID, MemberId: c.MemberID, ReviewerId: c.ReviewerID, LocalDate: apiDate(c.LocalDate),
		Status: api.CheckInStatus(c.Status), IsFinal: c.IsFinal, CutoffAt: c.CutoffAt, SubmitDeadline: c.SubmitDeadline,
		SubmittedAt: c.SubmittedAt, ReviewDeadline: c.ReviewDeadline, DecidedAt: c.DecidedAt,
		DisputeDeadline: c.DisputeDeadline, DisputedAt: c.DisputedAt, ResolutionDeadline: c.ResolutionDeadline,
		OverrideDeadline: c.OverrideDeadline, PenaltyApplied: c.PenaltyApplied,
	}
}

func apiProof(p store.Proof, attachments []store.Attachment) (api.Proof, error) {
	out := api.Proof{
		Id: p.ID, Version: int(p.Version), BodyText: p.BodyText, WordCount: int(p.WordCount),
		CreatedAt: p.CreatedAt, Links: []string{}, AttachmentIds: []openapi_types.UUID{},
	}
	if err := json.Unmarshal(p.BodyDoc, &out.BodyDoc); err != nil {
		return out, fmt.Errorf("proof %s body: %w", p.ID, err)
	}
	if len(p.Links) > 0 {
		if err := json.Unmarshal(p.Links, &out.Links); err != nil {
			return out, fmt.Errorf("proof %s links: %w", p.ID, err)
		}
	}
	for _, a := range attachments {
		out.AttachmentIds = append(out.AttachmentIds, a.ID)
	}
	return out, nil
}

func ptrInt(v *int32) *int {
	if v == nil {
		return nil
	}
	return ptr(int(*v))
}

// apiAttachment never fills `urls`; getAttachment adds them (they need the BlobStore).
func apiAttachment(a store.Attachment) api.Attachment {
	out := api.Attachment{
		Id: a.ID, PactId: a.PactID, Kind: api.AttachmentKind(a.Kind), Status: api.AttachmentStatus(a.Status),
		Mime: a.SniffedMime, Bytes: a.StoredBytes, Width: ptrInt(a.Width), Height: ptrInt(a.Height),
		DurationMs: ptrInt(a.DurationMs), RejectReason: a.RejectReason, CreatedAt: a.CreatedAt, ReadyAt: a.ReadyAt,
	}
	return out
}

func apiCheckInDetail(v service.CheckInView) (api.CheckInDetail, error) {
	out := api.CheckInDetail{
		CheckIn: apiCheckIn(v.CheckIn), PactTitle: v.PactTitle, Member: apiRef(v.Member), Reviewer: apiRef(v.Reviewer),
		ProofVersions: make([]api.ProofVersion, len(v.Versions)), Attachments: make([]api.Attachment, len(v.Attachments)),
		Decisions: make([]api.Decision, len(v.Decisions)), MyActions: make([]api.CheckInAction, len(v.Actions)),
		OverridesRemaining: v.OverridesRemaining,
	}
	for i, p := range v.Versions {
		out.ProofVersions[i] = api.ProofVersion{Id: p.ID, Version: int(p.Version), WordCount: int(p.WordCount), CreatedAt: p.CreatedAt}
	}
	for i, a := range v.Attachments {
		out.Attachments[i] = apiAttachment(a)
	}
	for i, d := range v.Decisions {
		out.Decisions[i] = api.Decision{
			Id: d.ID, Action: api.DecisionAction(d.Action), ActorId: d.ActorID, Reason: d.Reason,
			IsPower: d.IsPower, CreatedAt: d.CreatedAt,
		}
	}
	for i, a := range v.Actions {
		out.MyActions[i] = api.CheckInAction(a)
	}
	if v.Proof != nil {
		p, err := apiProof(*v.Proof, v.Attachments)
		if err != nil {
			return out, err
		}
		out.Proof = &p
	}
	return out, nil
}

func apiLedgerLine(l store.ListLedgerPageRow) api.LedgerLine {
	return api.LedgerLine{
		Id: l.ID, Kind: api.LedgerKind(l.Kind), Amount: l.Amount, BalanceAfter: l.BalanceAfter,
		CheckInId: l.CheckInID, CheckInLocalDate: apiDatePtr(l.CheckInLocalDate.Time, l.CheckInLocalDate.Valid),
		MemberId: l.CheckInMemberID, Note: l.Note, CreatedAt: l.CreatedAt,
	}
}

func apiLedgerLines(rows []store.ListLedgerPageRow) []api.LedgerLine {
	out := make([]api.LedgerLine, len(rows))
	for i, r := range rows {
		out[i] = apiLedgerLine(r)
	}
	return out
}

func apiNotification(n store.Notification) (api.Notification, error) {
	out := api.Notification{Id: n.ID, Kind: api.NotificationKind(n.Kind), Payload: map[string]interface{}{}, ReadAt: n.ReadAt, CreatedAt: n.CreatedAt}
	if len(n.Payload) > 0 {
		if err := json.Unmarshal(n.Payload, &out.Payload); err != nil {
			return out, fmt.Errorf("notification %d payload: %w", n.ID, err)
		}
	}
	return out, nil
}
