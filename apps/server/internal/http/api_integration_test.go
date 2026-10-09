//go:build integration

package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/andi-frame/lockedin/apps/server/internal/auth"
	"github.com/andi-frame/lockedin/apps/server/internal/config"
	"github.com/andi-frame/lockedin/apps/server/internal/domain"
	"github.com/andi-frame/lockedin/apps/server/internal/http/api"
	"github.com/andi-frame/lockedin/apps/server/internal/service"
	"github.com/andi-frame/lockedin/apps/server/internal/store"
	"github.com/andi-frame/lockedin/apps/server/internal/testdb"
)

// These tests drive the real stack (Fiber, middleware, handlers, services, Postgres,
// Redis) through HTTP only, the way the web app will.

var wib = time.FixedZone("WIB", 7*3600)

func at(day, hour, minute int) time.Time { return time.Date(2026, 11, day, hour, minute, 0, 0, wib) }

type stack struct {
	t     *testing.T
	app   *fiber.App
	st    *store.Store
	svc   *service.Service
	clock *domain.FakeClock
}

func newStack(t *testing.T) *stack { return buildStack(t, nil) }

// buildStack wires the real stack. uploads, when set, turns on the upload use-cases and mounts
// the fs driver's blob route, the way native dev runs.
func buildStack(t *testing.T, uploads *uploadWiring) *stack {
	t.Helper()
	st := testdb.New(t)
	rdb := testdb.RedisIn(t, 14)
	clock := domain.NewFakeClock(at(1, 9, 0))
	svc := service.New(st, clock)
	authSvc := auth.NewService(st, rdb, "integration-secret")
	deps := Deps{
		Config:   config.Config{Env: "staging", BaseURL: "http://localhost:3000"},
		Log:      discardLogger(),
		Redis:    rdb,
		Sessions: authSvc.Sessions(),
		Handlers: NewHandlers(svc, authSvc, false),
		Limits:   Limits{Global: 100000, Auth: 100000, Upload: 100000, Window: time.Minute},
	}
	if uploads != nil {
		svc.WithUploads(uploads.deps)
		deps.Blob, deps.BlobMaxBytes = uploads.blobs, 8<<20
	}
	return &stack{t: t, app: New(deps), st: st, svc: svc, clock: clock}
}

// client is one browser: it keeps the session and CSRF cookies like a cookie jar.
type client struct {
	s             *stack
	name          string
	id            uuid.UUID
	session, csrf string
}

type res struct {
	status int
	body   []byte
	header map[string][]string
}

func (r res) into(t *testing.T, v any) {
	t.Helper()
	if err := json.Unmarshal(r.body, v); err != nil {
		t.Fatalf("decode %T: %v\n%s", v, err, r.body)
	}
}

func (r res) problem(t *testing.T) api.Problem {
	t.Helper()
	var p api.Problem
	r.into(t, &p)
	return p
}

func (r res) hdr(name string) string {
	for k, v := range r.header {
		if strings.EqualFold(k, name) && len(v) > 0 {
			return v[0]
		}
	}
	return ""
}

func (c *client) call(method, path string, body any, headers ...string) res {
	c.s.t.Helper()
	var rd io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			c.s.t.Fatal(err)
		}
		rd = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.session != "" {
		req.Header.Set("Cookie", auth.SessionCookie+"="+c.session)
		if method != "GET" && method != "HEAD" {
			req.Header.Set(auth.CSRFHeader, c.csrf)
		}
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := c.s.app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
	if err != nil {
		c.s.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	for _, ck := range resp.Cookies() {
		val := ck.Value
		if ck.MaxAge < 0 {
			val = ""
		}
		switch ck.Name {
		case auth.SessionCookie:
			c.session = val
		case auth.CSRFCookie:
			c.csrf = val
		}
	}
	return res{status: resp.StatusCode, body: raw, header: resp.Header}
}

func (c *client) ok(want int, method, path string, body any, headers ...string) res {
	c.s.t.Helper()
	r := c.call(method, path, body, headers...)
	if r.status != want {
		c.s.t.Fatalf("%s %s %s: status %d, want %d\n%s", c.name, method, path, r.status, want, r.body)
	}
	return r
}

func (c *client) fails(status int, code api.ErrorCode, method, path string, body any, headers ...string) api.Problem {
	c.s.t.Helper()
	r := c.call(method, path, body, headers...)
	if r.status != status {
		c.s.t.Fatalf("%s %s %s: status %d, want %d\n%s", c.name, method, path, r.status, status, r.body)
	}
	p := r.problem(c.s.t)
	if p.Code != code {
		c.s.t.Fatalf("%s %s %s: code %q, want %q (%s)", c.name, method, path, p.Code, code, r.body)
	}
	return p
}

func (s *stack) anon() *client { return &client{s: s, name: "anon"} }

func (s *stack) register(name string) *client {
	s.t.Helper()
	c := &client{s: s, name: name}
	var u api.User
	c.ok(201, "POST", "/api/v1/auth/register", map[string]any{
		"email": strings.ToLower(name) + "@tepati.test", "password": "correct horse battery", "display_name": name,
	}).into(s.t, &u)
	c.id = u.Id
	return c
}

func i64(v int64) *int64 { return &v }

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func workedTerms(backer uuid.UUID) domain.Terms {
	return domain.Terms{
		Version: 1, Timezone: "Asia/Jakarta", StartsOn: domain.MustDate("2026-11-02"), EndsOn: domain.MustDate("2026-11-29"),
		CutoffLocalTime: "23:59", GraceMinutes: 30, CoinRateIDR: 1000, InitialPot: 1000, PotFloor: 0, PotCap: i64(1500),
		ReviewWindowHours: 24, DisputeWindowHours: 24, DisputeResolutionHours: 48, OverrideWindowHours: 48,
		MaxOverrides: 3, BackerCommits: true,
		Members: map[uuid.UUID]domain.MemberTerms{
			backer: {Role: domain.RoleBacker, Commitment: "Belajar Kalkulus 2 jam", Schedule: []int{1, 2, 3, 4, 5}, PenaltyPerMiss: 50, RestDays: 2},
			uuid.Nil: {Role: domain.RoleDoer, Commitment: "Latihan soal UTBK 50 soal", Schedule: []int{1, 2, 3, 4, 5, 6}, PenaltyPerMiss: 50, RestDays: 2,
				Evidence: domain.Evidence{MinWords: 20}},
		},
	}
}

func draftBody(t *testing.T, title string, terms domain.Terms) api.PactDraft {
	t.Helper()
	at, err := apiTerms(terms)
	if err != nil {
		t.Fatal(err)
	}
	return api.PactDraft{Title: title, Terms: at}
}

func words(n int) map[string]any {
	text := strings.TrimSpace(strings.Repeat("soal ", n))
	return map[string]any{"body_doc": map[string]any{
		"type": "doc", "content": []any{map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": text}}}},
	}}
}

type scene struct {
	s             *stack
	andi, bima    *client // backer, doer
	pact          uuid.UUID
	doerCheckIn   uuid.UUID // Bima, Mon 2 Nov
	backerCheckIn uuid.UUID // Andi, Mon 2 Nov
}

// activePact walks two users through the whole agreement over HTTP and activates the pact.
func (s *stack) activePact() scene {
	t := s.t
	t.Helper()
	andi, bima := s.register("Andi"), s.register("Bima")

	var p api.Pact
	andi.ok(201, "POST", "/api/v1/pacts", draftBody(t, "UTBK November", workedTerms(andi.id))).into(t, &p)
	var prop api.Proposal
	andi.ok(200, "POST", "/api/v1/pacts/"+p.Id.String()+"/propose", nil).into(t, &prop)
	bima.ok(200, "POST", "/api/v1/invites/"+prop.InviteToken+"/join", nil).into(t, &p)
	for _, c := range []*client{andi, bima} {
		c.ok(200, "POST", "/api/v1/pacts/"+p.Id.String()+"/accept", api.AcceptRequest{TermsHash: p.TermsHash, SignatureName: c.name}).into(t, &p)
	}
	if p.Status != api.Scheduled {
		t.Fatalf("pact = %s, want scheduled", p.Status)
	}
	s.clock.Set(at(2, 0, 0))
	if _, err := s.svc.ActivateDuePacts(context.Background()); err != nil { // the worker's job
		t.Fatal(err)
	}
	sc := scene{s: s, andi: andi, bima: bima, pact: p.Id}
	var list api.CheckInList
	bima.ok(200, "GET", "/api/v1/pacts/"+p.Id.String()+"/check-ins?from=2026-11-02&to=2026-11-02", nil).into(t, &list)
	for _, ci := range list.Items {
		if ci.MemberId == bima.id {
			sc.doerCheckIn = ci.Id
		} else {
			sc.backerCheckIn = ci.Id
		}
	}
	if sc.doerCheckIn == uuid.Nil || sc.backerCheckIn == uuid.Nil {
		t.Fatalf("missing Monday check-ins: %+v", list.Items)
	}
	return sc
}

func (sc scene) pactPath(suffix string) string  { return "/api/v1/pacts/" + sc.pact.String() + suffix }
func ciPath(id uuid.UUID, suffix string) string { return "/api/v1/check-ins/" + id.String() + suffix }

// ---------------------------------------------------------------- auth

func TestAuthLifecycleOverHTTP(t *testing.T) {
	s := newStack(t)
	c := &client{s: s, name: "Andi"}
	body := map[string]any{"email": "andi@tepati.test", "password": "correct horse battery", "display_name": "Andi"}

	r := c.ok(201, "POST", "/api/v1/auth/register", body)
	var u api.User
	r.into(t, &u)
	if u.DisplayName != "Andi" || u.Locale != "id" || u.Timezone != "Asia/Jakarta" || string(u.Email) != "andi@tepati.test" {
		t.Fatalf("registered user = %+v", u)
	}
	if c.session == "" || c.csrf == "" {
		t.Fatal("register must set both the session and the CSRF cookies")
	}
	c.ok(200, "GET", "/api/v1/me", nil)

	s.anon().fails(409, api.AuthEmailTaken, "POST", "/api/v1/auth/register", body)
	s.anon().fails(400, api.AuthWeakPassword, "POST", "/api/v1/auth/register", map[string]any{"email": "x@tepati.test", "password": "short", "display_name": "X"})
	s.anon().fails(401, api.AuthInvalidCredentials, "POST", "/api/v1/auth/login", map[string]any{"email": "andi@tepati.test", "password": "wrong password!"})
	s.anon().fails(401, api.AuthInvalidCredentials, "POST", "/api/v1/auth/login", map[string]any{"email": "nobody@tepati.test", "password": "wrong password!"})

	// CSRF: an unsafe request with a valid session but no matching token is refused.
	forged := &client{s: s, name: "forger", session: c.session, csrf: "forged"}
	forged.fails(403, api.AuthCsrf, "POST", "/api/v1/auth/logout", nil)

	second := &client{s: s, name: "Andi (another device)"}
	second.ok(200, "POST", "/api/v1/auth/login", map[string]any{"email": "ANDI@tepati.test", "password": "correct horse battery"})
	if second.session == "" || second.session == c.session {
		t.Fatal("login must start a new session")
	}

	c.ok(204, "POST", "/api/v1/auth/logout", nil)
	if c.session != "" {
		t.Error("logout must clear the cookies")
	}
	stale := &client{s: s, name: "stale", session: "whatever", csrf: "x"}
	stale.fails(401, api.AuthUnauthenticated, "GET", "/api/v1/me", nil)
	second.ok(200, "GET", "/api/v1/me", nil) // other devices stay signed in
}

// PLAN 9.4: PATCH /me changes only what is sent, refuses what cannot be switched off, and needs a
// session and the CSRF token like every other unsafe request.
func TestUpdateMeOverHTTP(t *testing.T) {
	s := newStack(t)
	c := &client{s: s, name: "Sari"}
	var u api.User
	c.ok(201, "POST", "/api/v1/auth/register", map[string]any{"email": "sari@tepati.test", "password": "correct horse battery", "display_name": "Sari"}).into(t, &u)
	if u.EmailKindsOff == nil || len(u.EmailKindsOff) != 0 {
		t.Fatalf("a new user has every email on, as an empty list (not null): %#v", u.EmailKindsOff)
	}

	c.ok(200, "PATCH", "/api/v1/me", map[string]any{"display_name": " Sari Dewi ", "locale": "en", "email_kinds_off": []string{"terms_signed", "proof_rejected"}}).into(t, &u)
	if u.DisplayName != "Sari Dewi" || u.Locale != "en" || u.Timezone != "Asia/Jakarta" || len(u.EmailKindsOff) != 2 || u.EmailKindsOff[0] != "proof_rejected" {
		t.Fatalf("after PATCH: %+v", u)
	}
	var again api.User
	c.ok(200, "GET", "/api/v1/me", nil).into(t, &again)
	if again.DisplayName != "Sari Dewi" || len(again.EmailKindsOff) != 2 {
		t.Fatalf("GET after PATCH: %+v", again)
	}
	c.ok(200, "PATCH", "/api/v1/me", map[string]any{"timezone": "Asia/Makassar"}).into(t, &u)
	if u.Timezone != "Asia/Makassar" || len(u.EmailKindsOff) != 2 {
		t.Fatalf("a PATCH without email_kinds_off must keep the list: %+v", u)
	}

	c.fails(400, api.AuthInvalidName, "PATCH", "/api/v1/me", map[string]any{"display_name": "  "})
	c.fails(400, api.ValidationFailed, "PATCH", "/api/v1/me", map[string]any{"timezone": "Mars/Olympus"})
	c.fails(400, api.ValidationFailed, "PATCH", "/api/v1/me", map[string]any{"locale": "fr"})
	c.fails(400, api.ValidationFailed, "PATCH", "/api/v1/me", map[string]any{"email_kinds_off": []string{"dispute_opened"}})

	s.anon().fails(401, api.AuthUnauthenticated, "PATCH", "/api/v1/me", map[string]any{"display_name": "X"})
	forged := &client{s: s, name: "forger", session: c.session, csrf: "forged"}
	forged.fails(403, api.AuthCsrf, "PATCH", "/api/v1/me", map[string]any{"display_name": "X"})
}

// ---------------------------------------------------------------- pact onboarding

func TestPactOnboardingOverHTTP(t *testing.T) {
	s := newStack(t)
	andi, bima := s.register("Andi"), s.register("Bima")

	var p api.Pact
	andi.ok(201, "POST", "/api/v1/pacts", draftBody(t, "UTBK November", workedTerms(andi.id))).into(t, &p)
	if p.Status != api.Draft || p.MyRole != api.RoleBacker || len(p.Members) != 1 || p.Balance != 0 {
		t.Fatalf("draft = %+v", p)
	}
	if _, ok := p.Terms.Members[uuid.Nil.String()]; !ok {
		t.Error("the doer slot should be keyed by the nil UUID until someone joins")
	}

	// Validation: bad terms come back as pact.invalid_terms naming the rule; an empty title as validation.failed.
	bad := workedTerms(andi.id)
	bad.InitialPot = 0
	pr := andi.fails(400, api.PactInvalidTerms, "POST", "/api/v1/pacts", draftBody(t, "x", bad))
	if pr.Detail == nil || !strings.Contains(*pr.Detail, "initial_pot") {
		t.Errorf("detail should name the field: %v", pr.Detail)
	}
	andi.fails(400, api.ValidationFailed, "POST", "/api/v1/pacts", draftBody(t, "   ", workedTerms(andi.id)))

	// SPEC §3: a start date that has already passed (the stack's clock is on 2026-11-01) is a 409 of its own.
	late := workedTerms(andi.id)
	late.StartsOn, late.EndsOn = domain.MustDate("2026-10-20"), domain.MustDate("2026-11-20")
	andi.fails(409, api.PactStartPassed, "POST", "/api/v1/pacts", draftBody(t, "Lama", late))

	edited := workedTerms(andi.id)
	d := edited.Members[uuid.Nil]
	d.PenaltyPerMiss = 75
	edited.Members[uuid.Nil] = d
	var after api.Pact
	andi.ok(200, "PATCH", "/api/v1/pacts/"+p.Id.String(), draftBody(t, "UTBK November (revisi)", edited)).into(t, &after)
	if after.TermsHash == p.TermsHash || after.TermsVersion != p.TermsVersion+1 || after.Title != "UTBK November (revisi)" {
		t.Errorf("editing must change the hash and bump the version: %+v", after)
	}

	var prop api.Proposal
	andi.ok(200, "POST", "/api/v1/pacts/"+p.Id.String()+"/propose", nil).into(t, &prop)

	// The invitee previews without an account.
	var prev api.InvitePreview
	s.anon().ok(200, "GET", "/api/v1/invites/"+prop.InviteToken, nil).into(t, &prev)
	if prev.PactTitle != "UTBK November (revisi)" || prev.Inviter.DisplayName != "Andi" || !prev.DoerSlotOpen || prev.TermsHash != after.TermsHash {
		t.Errorf("preview = %+v", prev)
	}
	s.anon().fails(410, api.PactInviteInvalid, "GET", "/api/v1/invites/not-a-real-token-at-all", nil)

	bima.ok(200, "POST", "/api/v1/invites/"+prop.InviteToken+"/join", nil).into(t, &p)
	if len(p.Members) != 2 || p.MyRole != api.RoleDoer || p.TermsHash == after.TermsHash {
		t.Fatalf("after joining: %+v", p)
	}
	bima.fails(410, api.PactInviteInvalid, "POST", "/api/v1/invites/"+prop.InviteToken+"/join", nil) // consumed

	bima.fails(409, api.PactTermsMismatch, "POST", "/api/v1/pacts/"+p.Id.String()+"/accept", api.AcceptRequest{TermsHash: after.TermsHash, SignatureName: "Bima"})
	bima.fails(422, api.PactSignatureMismatch, "POST", "/api/v1/pacts/"+p.Id.String()+"/accept", api.AcceptRequest{TermsHash: p.TermsHash, SignatureName: "Orang Lain"})
	bima.fails(400, api.ValidationFailed, "POST", "/api/v1/pacts/"+p.Id.String()+"/accept", api.AcceptRequest{TermsHash: "abc", SignatureName: "Bima"})

	bima.ok(200, "POST", "/api/v1/pacts/"+p.Id.String()+"/accept", api.AcceptRequest{TermsHash: p.TermsHash, SignatureName: "bima"}).into(t, &p)
	bima.ok(200, "POST", "/api/v1/pacts/"+p.Id.String()+"/accept", api.AcceptRequest{TermsHash: p.TermsHash, SignatureName: "Bima"}) // idempotent
	if p.Status != api.Proposed {
		t.Fatalf("one signature is not enough: %s", p.Status)
	}
	andi.ok(200, "POST", "/api/v1/pacts/"+p.Id.String()+"/accept", api.AcceptRequest{TermsHash: p.TermsHash, SignatureName: "Andi"}).into(t, &p)
	if p.Status != api.Scheduled || p.Balance != 1000 {
		t.Fatalf("both signed: status %s balance %d", p.Status, p.Balance)
	}
	for _, m := range p.Members {
		if !m.Accepted {
			t.Errorf("%s should show as accepted", m.DisplayName)
		}
	}
	andi.fails(409, api.PactInvalidState, "PATCH", "/api/v1/pacts/"+p.Id.String(), draftBody(t, "Terlambat", workedTerms(andi.id)))

	var ci api.CheckInList
	andi.ok(200, "GET", "/api/v1/pacts/"+p.Id.String()+"/check-ins", nil).into(t, &ci)
	if len(ci.Items) != 24+20 {
		t.Errorf("check-ins = %d, want 44", len(ci.Items))
	}
}

func TestPactListPagination(t *testing.T) {
	s := newStack(t)
	andi := s.register("Andi")
	for _, title := range []string{"A", "B", "C"} {
		andi.ok(201, "POST", "/api/v1/pacts", draftBody(t, title, workedTerms(andi.id)))
	}
	var page api.PactPage
	andi.ok(200, "GET", "/api/v1/pacts?limit=2", nil).into(t, &page)
	if len(page.Items) != 2 || page.NextCursor == nil || page.Items[0].Title != "C" {
		t.Fatalf("page 1 = %d items, cursor %v", len(page.Items), page.NextCursor)
	}
	var rest api.PactPage
	andi.ok(200, "GET", "/api/v1/pacts?limit=2&cursor="+*page.NextCursor, nil).into(t, &rest)
	if len(rest.Items) != 1 || rest.NextCursor != nil || rest.Items[0].Title != "A" {
		t.Fatalf("page 2 = %+v", rest)
	}
	andi.fails(400, api.ValidationFailed, "GET", "/api/v1/pacts?cursor=%25%25not-base64", nil)
}

// ---------------------------------------------------------------- check-in flow

func TestCheckInFlowOverHTTP(t *testing.T) {
	s := newStack(t)
	sc := s.activePact()
	s.clock.Set(at(2, 10, 0))

	// Today for the doer.
	var today api.Today
	sc.bima.ok(200, "GET", "/api/v1/today", nil).into(t, &today)
	if len(today.MyCheckIns) != 1 || len(today.Pacts) != 1 || today.Pacts[0].Balance != 1000 || today.Pacts[0].Partner.DisplayName != "Andi" {
		t.Fatalf("today = %+v", today)
	}

	// Evidence rules: Bima must write 20 words.
	sc.bima.fails(422, api.CheckinEvidenceInsufficient, "PUT", ciPath(sc.doerCheckIn, "/proof"), words(5))
	sc.bima.fails(400, api.ProofInvalidDoc, "PUT", ciPath(sc.doerCheckIn, "/proof"), map[string]any{"body_doc": map[string]any{"type": "doc", "content": []any{map[string]any{"type": "script"}}}})

	key := []string{"Idempotency-Key", "submit-2026-11-02-bima"}
	var d api.CheckInDetail
	first := sc.bima.ok(200, "PUT", ciPath(sc.doerCheckIn, "/proof"), words(25), key...)
	first.into(t, &d)
	if d.CheckIn.Status != api.CheckInStatusSubmitted || d.Proof == nil || d.Proof.WordCount != 25 || len(d.ProofVersions) != 1 {
		t.Fatalf("after submit: %+v", d)
	}
	if !has(d.MyActions, api.CheckInActionEditProof) {
		t.Errorf("the doer may still edit: %v", d.MyActions)
	}
	replay := sc.bima.ok(200, "PUT", ciPath(sc.doerCheckIn, "/proof"), words(25), key...)
	if replay.hdr("Idempotent-Replayed") != "true" || !bytes.Equal(replay.body, first.body) {
		t.Error("a retry with the same Idempotency-Key must replay the first response")
	}
	sc.bima.ok(200, "GET", ciPath(sc.doerCheckIn, ""), nil).into(t, &d)
	if len(d.ProofVersions) != 1 {
		t.Errorf("the replay must not create a second proof version, got %d", len(d.ProofVersions))
	}

	// Review: nobody reviews their own check-in; the backer sees it in the queue.
	sc.bima.fails(403, api.CheckinNotAllowed, "POST", ciPath(sc.doerCheckIn, "/approve"), nil)
	var q api.ReviewQueuePage
	sc.andi.ok(200, "GET", "/api/v1/review-queue", nil).into(t, &q)
	if len(q.Items) != 1 || q.Items[0].CheckIn.Id != sc.doerCheckIn || q.Items[0].WordCount != 25 || q.Items[0].Member.DisplayName != "Bima" {
		t.Fatalf("review queue = %+v", q)
	}
	sc.bima.ok(200, "GET", "/api/v1/review-queue", nil).into(t, &q)
	if len(q.Items) != 0 {
		t.Error("the doer reviews nothing")
	}

	sc.andi.fails(400, api.CheckinReasonRequired, "POST", ciPath(sc.doerCheckIn, "/reject"), map[string]any{"reason": "kurang"})
	sc.andi.ok(200, "POST", ciPath(sc.doerCheckIn, "/approve"), nil).into(t, &d)
	if d.CheckIn.Status != api.CheckInStatusApproved || !d.CheckIn.IsFinal {
		t.Fatalf("after approve: %+v", d.CheckIn)
	}
	last := d.Decisions[len(d.Decisions)-1]
	if last.Action != api.DecisionActionApprove || last.IsPower || last.ActorId == nil || *last.ActorId != sc.andi.id {
		t.Errorf("last decision = %+v", last)
	}
	sc.andi.fails(409, api.CheckinInvalidTransition, "POST", ciPath(sc.doerCheckIn, "/approve"), nil)

	// Rest days and the backer's own check-in.
	other := s.nextCheckIn(sc, sc.bima.id, 3)
	sc.bima.ok(200, "POST", ciPath(other, "/rest"), nil).into(t, &d)
	if d.CheckIn.Status != api.CheckInStatusRest {
		t.Errorf("rest = %s", d.CheckIn.Status)
	}
}

func has(actions []api.CheckInAction, a api.CheckInAction) bool {
	for _, x := range actions {
		if x == a {
			return true
		}
	}
	return false
}

func (s *stack) nextCheckIn(sc scene, member uuid.UUID, day int) uuid.UUID {
	s.t.Helper()
	var list api.CheckInList
	d := fmt.Sprintf("2026-11-%02d", day)
	sc.andi.ok(200, "GET", sc.pactPath("/check-ins?from="+d+"&to="+d), nil).into(s.t, &list)
	for _, ci := range list.Items {
		if ci.MemberId == member {
			return ci.Id
		}
	}
	s.t.Fatalf("no check-in for %s on %s", member, d)
	return uuid.Nil
}

func TestRejectDisputeAndPowerOverHTTP(t *testing.T) {
	s := newStack(t)
	sc := s.activePact()
	s.clock.Set(at(2, 10, 0))
	sc.bima.ok(200, "PUT", ciPath(sc.doerCheckIn, "/proof"), words(25))
	sc.andi.ok(200, "POST", ciPath(sc.doerCheckIn, "/reject"), map[string]any{"reason": "buktinya belum cukup jelas"})

	var d api.CheckInDetail
	sc.bima.ok(200, "GET", ciPath(sc.doerCheckIn, ""), nil).into(t, &d)
	if !has(d.MyActions, api.CheckInActionDispute) || d.OverridesRemaining != nil {
		t.Fatalf("doer after a rejection: actions %v, overrides %v", d.MyActions, d.OverridesRemaining)
	}
	sc.andi.fails(403, api.CheckinNotAllowed, "POST", ciPath(sc.doerCheckIn, "/dispute"), map[string]any{"reason": "saya tidak setuju sama sekali"})
	sc.bima.ok(200, "POST", ciPath(sc.doerCheckIn, "/dispute"), map[string]any{"reason": "semua soal sudah saya kirim"})

	// The backer's rationale is mandatory for uphold too (SPEC §2), even though the state machine would accept none.
	sc.andi.fails(400, api.CheckinReasonRequired, "POST", ciPath(sc.doerCheckIn, "/dispute/resolve"), map[string]any{"outcome": "uphold", "reason": "ok"})
	sc.bima.fails(403, api.CheckinNotAllowed, "POST", ciPath(sc.doerCheckIn, "/dispute/resolve"), map[string]any{"outcome": "uphold", "reason": "saya menang sendiri ya"})
	sc.andi.ok(200, "POST", ciPath(sc.doerCheckIn, "/dispute/resolve"), map[string]any{"outcome": "uphold", "reason": "setelah dilihat lagi, sudah cukup"}).into(t, &d)
	if d.CheckIn.Status != api.CheckInStatusApproved {
		t.Fatalf("upheld dispute: %s", d.CheckIn.Status)
	}
	last := d.Decisions[len(d.Decisions)-1]
	if last.Action != api.DecisionActionUphold || !last.IsPower {
		t.Errorf("uphold must be flagged as a power action: %+v", last)
	}
	var bal api.LedgerPage
	sc.bima.ok(200, "GET", sc.pactPath("/ledger"), nil).into(t, &bal)
	if bal.Balance != 1000 || len(bal.Items) != 1 {
		t.Errorf("an upheld dispute moves no coins: %+v", bal)
	}
	sc.andi.fails(400, api.ValidationFailed, "POST", ciPath(sc.doerCheckIn, "/dispute/resolve"), map[string]any{"outcome": "maybe", "reason": "alasan yang cukup panjang"})
}

// ---------------------------------------------------------------- settlement

func TestSettlementOverHTTP(t *testing.T) {
	s := newStack(t)
	sc := s.activePact()
	ctx := context.Background()

	// The doer's first day goes well; everything else lapses and the worker settles the pact.
	s.clock.Set(at(2, 10, 0))
	sc.bima.ok(200, "PUT", ciPath(sc.doerCheckIn, "/proof"), words(25))
	sc.andi.ok(200, "POST", ciPath(sc.doerCheckIn, "/approve"), nil)
	s.clock.Set(at(30, 12, 0))
	for range 5 {
		if _, err := s.svc.SweepDeadlines(ctx, 500); err != nil {
			t.Fatal(err)
		}
	}
	if closed, err := s.svc.ClosePacts(ctx, 10); err != nil || len(closed) != 1 {
		t.Fatalf("close: %v %v", closed, err)
	}

	var p api.Pact
	sc.bima.ok(200, "GET", sc.pactPath(""), nil).into(t, &p)
	if p.Status != api.Settling || p.Payout == nil || p.Payout.Amount < 0 {
		t.Fatalf("settling pact = %+v", p)
	}
	amount := p.Payout.Amount

	// Roles are enforced with distinct codes.
	sc.bima.fails(403, api.PactNotBacker, "POST", sc.pactPath("/payout/mark-paid"), nil)
	sc.andi.fails(403, api.PactNotDoer, "POST", sc.pactPath("/payout/confirm"), nil)

	var po api.Payout
	sc.andi.ok(200, "POST", sc.pactPath("/payout/mark-paid"), map[string]any{"note": "transfer lewat dana"}).into(t, &po)
	if po.MarkedPaidAt == nil || po.MarkedPaidNote == nil || po.ConfirmedAt != nil || po.Amount != amount {
		t.Fatalf("after mark-paid: %+v", po)
	}
	sc.bima.ok(200, "POST", sc.pactPath("/payout/confirm"), nil).into(t, &po)
	if po.ConfirmedAt == nil {
		t.Fatalf("after confirm: %+v", po)
	}
	sc.bima.ok(200, "GET", sc.pactPath(""), nil).into(t, &p)
	if p.Status != api.Completed || p.CompletedAt == nil {
		t.Errorf("pact = %s", p.Status)
	}
	sc.andi.fails(409, api.PactInvalidState, "POST", sc.pactPath("/payout/mark-paid"), nil)

	// The passbook pages newest first with a running balance that agrees across pages.
	var page1, page2 api.LedgerPage
	sc.bima.ok(200, "GET", sc.pactPath("/ledger?limit=3"), nil).into(t, &page1)
	if len(page1.Items) != 3 || page1.NextCursor == nil || page1.Items[0].Kind != api.LedgerKindPayout || page1.Items[0].BalanceAfter != 0 || page1.Balance != 0 {
		t.Fatalf("ledger page 1 = %+v", page1)
	}
	sc.bima.ok(200, "GET", sc.pactPath("/ledger?limit=100&cursor="+*page1.NextCursor), nil).into(t, &page2)
	if page2.NextCursor != nil || page2.Items[len(page2.Items)-1].Kind != api.LedgerKindPotInitial {
		t.Errorf("ledger page 2 should end at the initial pot: %d items, cursor %v", len(page2.Items), page2.NextCursor)
	}
	sc.bima.fails(400, api.ValidationFailed, "GET", sc.pactPath("/ledger?cursor=garbage"), nil)
}

// ---------------------------------------------------------------- notifications

func TestNotificationsOverHTTP(t *testing.T) {
	s := newStack(t)
	andi, bima := s.register("Andi"), s.register("Bima")
	ctx := context.Background()
	for _, k := range []string{"proof_rejected", "day_missed", "pact_settled"} {
		if _, err := s.st.InsertNotification(ctx, store.InsertNotificationParams{UserID: bima.id, Kind: k, Payload: []byte(`{"pact_id":"` + uuid.NewString() + `"}`)}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.st.InsertNotification(ctx, store.InsertNotificationParams{UserID: andi.id, Kind: "proof_submitted", Payload: []byte(`{}`)}); err != nil {
		t.Fatal(err)
	}

	var page api.NotificationPage
	bima.ok(200, "GET", "/api/v1/notifications?limit=2", nil).into(t, &page)
	if len(page.Items) != 2 || page.NextCursor == nil || page.UnreadCount != 3 || page.Items[0].Kind != api.NotificationKindPactSettled {
		t.Fatalf("page 1 = %+v", page)
	}
	if page.Items[0].Payload["pact_id"] == nil {
		t.Error("the payload should carry the pact id for deep links")
	}

	var theirs api.NotificationPage
	andi.ok(200, "GET", "/api/v1/notifications", nil).into(t, &theirs)
	// Bima cannot mark Andi's notification, only his own.
	bima.ok(204, "POST", "/api/v1/notifications/read", map[string]any{"ids": []int64{page.Items[0].Id, theirs.Items[0].Id}})
	bima.ok(200, "GET", "/api/v1/notifications?unread_only=true", nil).into(t, &page)
	if len(page.Items) != 2 || page.UnreadCount != 2 {
		t.Errorf("unread = %+v", page)
	}
	andi.ok(200, "GET", "/api/v1/notifications", nil).into(t, &theirs)
	if theirs.UnreadCount != 1 {
		t.Errorf("Andi's notification must stay unread, got %d", theirs.UnreadCount)
	}
	bima.fails(400, api.ValidationFailed, "POST", "/api/v1/notifications/read", map[string]any{"ids": []int64{}})
}

// ---------------------------------------------------------------- authorization

func TestNonMembersGetNotFoundEverywhere(t *testing.T) {
	s := newStack(t)
	sc := s.activePact()
	eka := s.register("Eka")
	random := uuid.NewString()
	reason := map[string]any{"reason": "alasan yang cukup panjang"}
	hash := strings.Repeat("a", 64)

	type op struct {
		name, method, path, unknownPath, template string
		body                                      any
	}
	pact := func(suffix string) (string, string) { return sc.pactPath(suffix), "/api/v1/pacts/" + random + suffix }
	ci := func(suffix string) (string, string) {
		return ciPath(sc.doerCheckIn, suffix), ciPath(uuid.MustParse(random), suffix)
	}
	mk := func(name, method string, paths func(string) (string, string), base, suffix string, body any) op {
		real, unknown := paths(suffix)
		return op{name, method, real, unknown, base + suffix, body}
	}
	const pactBase, ciBase = "/pacts/{pactId}", "/check-ins/{checkInId}"
	ops := []op{
		mk("get pact", "GET", pact, pactBase, "", nil),
		mk("update pact", "PATCH", pact, pactBase, "", draftBody(t, "x", workedTerms(eka.id))),
		mk("propose", "POST", pact, pactBase, "/propose", nil),
		mk("accept", "POST", pact, pactBase, "/accept", map[string]any{"terms_hash": hash, "signature_name": "Eka"}),
		mk("list check-ins", "GET", pact, pactBase, "/check-ins", nil),
		mk("ledger", "GET", pact, pactBase, "/ledger", nil),
		mk("mark paid", "POST", pact, pactBase, "/payout/mark-paid", nil),
		mk("confirm payout", "POST", pact, pactBase, "/payout/confirm", nil),
		mk("get check-in", "GET", ci, ciBase, "", nil),
		mk("submit proof", "PUT", ci, ciBase, "/proof", words(25)),
		mk("declare rest", "POST", ci, ciBase, "/rest", nil),
		mk("approve", "POST", ci, ciBase, "/approve", nil),
		mk("reject", "POST", ci, ciBase, "/reject", reason),
		mk("override", "POST", ci, ciBase, "/override", reason),
		mk("dispute", "POST", ci, ciBase, "/dispute", reason),
		mk("resolve dispute", "POST", ci, ciBase, "/dispute/resolve", map[string]any{"outcome": "uphold", "reason": "alasan yang cukup panjang"}),
	}
	// The table must be exactly the contract's pact- and check-in-scoped operations (spec_test.go keeps the list).
	covered := map[string]bool{}
	for _, o := range ops {
		covered[o.method+" "+o.template] = true
	}
	for key, where := range scopedOperations {
		if where == "table" && !covered[key] {
			t.Errorf("%s is in scopedOperations as covered by this table but is missing from it", key)
		}
	}
	for key := range covered {
		if scopedOperations[key] != "table" {
			t.Errorf("%s is in this table but not marked \"table\" in scopedOperations", key)
		}
	}
	for _, o := range ops {
		t.Run(o.name, func(t *testing.T) {
			real := eka.call(o.method, o.path, o.body)
			unknown := eka.call(o.method, o.unknownPath, o.body)
			if real.status != 404 || real.problem(t).Code != api.NotFound {
				t.Fatalf("a non-member must get 404 not_found, got %d %s", real.status, real.body)
			}
			// Indistinguishable from a pact or check-in that does not exist, so existence never leaks.
			if unknown.status != real.status || unknown.problem(t).Code != real.problem(t).Code {
				t.Errorf("non-member response differs from the unknown-id response: %d vs %d", real.status, unknown.status)
			}
			if anon := s.anon().call(o.method, o.path, o.body); anon.status != 401 {
				t.Errorf("anonymous callers must get 401, got %d", anon.status)
			}
		})
	}

	// Members who lack the power get 403 with a code that says why, never a silent no-op.
	sc.bima.fails(403, api.PactNotBacker, "POST", sc.pactPath("/propose"), nil)
	// Either member may edit terms while proposed (SPEC §3), but never once the pact is running.
	sc.bima.fails(409, api.PactInvalidState, "PATCH", sc.pactPath(""), draftBody(t, "x", workedTerms(sc.andi.id)))
	sc.andi.fails(403, api.CheckinNotAllowed, "PUT", ciPath(sc.doerCheckIn, "/proof"), words(25))

	// A stranger's token-less probes.
	eka.fails(410, api.PactInviteInvalid, "POST", "/api/v1/invites/"+strings.Repeat("z", 32)+"/join", nil)
	var list api.PactPage
	eka.ok(200, "GET", "/api/v1/pacts", nil).into(t, &list)
	if len(list.Items) != 0 {
		t.Errorf("a stranger's pact list must be empty, got %d", len(list.Items))
	}
}

func TestUploadOperationsAnswer503WhenStorageIsNotConfigured(t *testing.T) {
	s := newStack(t)
	sc := s.activePact()
	body := map[string]any{"pact_id": sc.pact.String(), "kind": "image", "mime": "image/webp", "bytes": 1000}
	sc.bima.fails(503, api.ServerUnavailable, "POST", "/api/v1/uploads", body)
	sc.bima.fails(503, api.ServerUnavailable, "POST", "/api/v1/uploads/"+uuid.NewString()+"/complete", nil)
	sc.bima.fails(503, api.ServerUnavailable, "GET", "/api/v1/attachments/"+uuid.NewString(), nil)
}

// ---------------------------------------------------------------- contract coverage

// Every operation in api/openapi.yaml must be routed, and nothing outside it may be.
func TestEveryContractOperationIsRouted(t *testing.T) {
	s := newStack(t)
	doc := loadSpec(t)
	placeholder := regexp.MustCompile(`\{([^}]+)\}`)

	routed := map[string]bool{}
	for _, r := range s.app.GetRoutes(true) {
		routed[r.Method+" "+r.Path] = true
	}
	want := map[string]bool{"GET /healthz": true, "GET /readyz": true, "GET /metrics": true}
	for _, op := range doc.operations(t) {
		key := op.Method + " " + basePath + placeholder.ReplaceAllString(op.Path, ":$1")
		want[key] = true
		if !routed[key] {
			t.Errorf("operation %s %s (%s) has no route", op.Method, op.Path, op.Op.OperationID)
		}
	}
	for key := range routed {
		if strings.HasPrefix(key, "HEAD ") || strings.HasPrefix(key, "OPTIONS ") {
			continue
		}
		if !want[key] {
			t.Errorf("route %q is not in the contract", key)
		}
	}
}
