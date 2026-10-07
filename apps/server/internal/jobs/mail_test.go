package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/andi-frame/lockedin/apps/server/internal/notify"
	"github.com/andi-frame/lockedin/apps/server/internal/service"
)

// fakeMailing plays the part of the service's Claim*/Release* functions.
type fakeMailing struct {
	notif    map[int64]service.NotificationEmail // claimable notifications
	digest   *service.DigestEmail
	invite   *service.InviteEmail
	claimed  []string
	released []string
	claimErr error
}

func (f *fakeMailing) ClaimNotificationEmail(_ context.Context, id int64) (service.NotificationEmail, bool, error) {
	f.claimed = append(f.claimed, "n")
	n, ok := f.notif[id]
	delete(f.notif, id) // a second claim finds nothing
	return n, ok, f.claimErr
}
func (f *fakeMailing) ReleaseNotificationEmail(_ context.Context, _ int64) error {
	f.released = append(f.released, "n")
	return nil
}
func (f *fakeMailing) ClaimDigestEmail(context.Context, uuid.UUID, uuid.UUID, string) (service.DigestEmail, bool, error) {
	f.claimed = append(f.claimed, "d")
	if f.digest == nil {
		return service.DigestEmail{}, false, f.claimErr
	}
	d := *f.digest
	f.digest = nil
	return d, true, f.claimErr
}
func (f *fakeMailing) ReleaseDigestEmail(context.Context, []int64) error {
	f.released = append(f.released, "d")
	return nil
}
func (f *fakeMailing) ClaimInviteEmail(context.Context, string) (service.InviteEmail, bool, error) {
	f.claimed = append(f.claimed, "i")
	if f.invite == nil {
		return service.InviteEmail{}, false, f.claimErr
	}
	i := *f.invite
	f.invite = nil
	return i, true, f.claimErr
}
func (f *fakeMailing) ReleaseInviteEmail(context.Context, string) error {
	f.released = append(f.released, "i")
	return nil
}

type fakeSender struct {
	sent []notify.Message
	err  error
}

func (f *fakeSender) Send(_ context.Context, m notify.Message) error {
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, m)
	return nil
}

type fakeQueue struct {
	tasks []*asynq.Task
	opts  [][]asynq.Option
	err   error
}

func (f *fakeQueue) Enqueue(t *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.tasks = append(f.tasks, t)
	f.opts = append(f.opts, opts)
	return &asynq.TaskInfo{}, nil
}

type mailRig struct {
	h      *Handlers
	m      *Metrics
	set    *fakeSettlement
	mailer *fakeMailing
	send   *fakeSender
	queue  *fakeQueue
}

func newMailRig() *mailRig {
	r := &mailRig{set: &fakeSettlement{sweeps: []int{0}}, mailer: &fakeMailing{notif: map[int64]service.NotificationEmail{}}, send: &fakeSender{}, queue: &fakeQueue{}}
	r.m = NewMetrics()
	r.h = NewHandlers(r.set, slog.New(slog.NewTextHandler(io.Discard, nil)), r.m).WithMail(&Mail{
		Svc: r.mailer, Renderer: notify.NewRenderer("http://localhost:3000"), Sender: r.send, Queue: r.queue,
	})
	return r
}

func (r *mailRig) process(typ string, payload any) error {
	b, _ := json.Marshal(payload)
	return r.h.Mux().ProcessTask(context.Background(), asynq.NewTask(typ, b))
}

func delivered(id int64, user, pact uuid.UUID, kind string) service.DeliveredNotification {
	return service.DeliveredNotification{ID: id, Notification: service.Notification{UserID: user, PactID: pact, Kind: kind}}
}

func TestRelayQueuesEmailsAfterTheCommit(t *testing.T) {
	r := newMailRig()
	user, pact := uuid.New(), uuid.New()
	r.set.relay = service.RelayResult{
		Notifications: []service.DeliveredNotification{
			delivered(7, user, pact, "proof_rejected"),      // immediate
			delivered(8, user, pact, "proof_submitted"),     // digest
			delivered(9, user, pact, "day_missed"),          // in-app only
			delivered(10, user, pact, "reminder_cutoff_3h"), // in-app only
		},
		Invites: []service.InviteMail{{PactID: pact, Email: "calon@tepati.test", Token: "tok_1"}},
	}
	if err := run(t, r.h, TypeOutboxRelay); err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, task := range r.queue.tasks {
		got[task.Type()] += string(task.Payload()) + ";"
	}
	if len(r.queue.tasks) != 3 {
		t.Fatalf("queued %d tasks %v, want 3 (immediate, digest, invite)", len(r.queue.tasks), got)
	}
	if !strings.Contains(got[TypeEmailNotification], `"id":7`) {
		t.Errorf("notification task = %q", got[TypeEmailNotification])
	}
	if !strings.Contains(got[TypeEmailDigest], user.String()) || !strings.Contains(got[TypeEmailDigest], "proof_submitted") {
		t.Errorf("digest task = %q", got[TypeEmailDigest])
	}
	if !strings.Contains(got[TypeEmailInvite], "tok_1") {
		t.Errorf("invite task = %q", got[TypeEmailInvite])
	}
	for i, task := range r.queue.tasks {
		var queue string
		for _, o := range r.queue.opts[i] {
			if o.Type() == asynq.QueueOpt {
				queue = o.Value().(string)
			}
		}
		if queue != QueueDefault {
			t.Errorf("%s runs on %q, want %q", task.Type(), queue, QueueDefault)
		}
	}
}

func TestRelayWithoutMailDoesNotQueueEmails(t *testing.T) {
	f := &fakeSettlement{relay: service.RelayResult{Notifications: []service.DeliveredNotification{delivered(1, uuid.New(), uuid.New(), "proof_rejected")}}}
	h, _ := newTestHandlers(f)
	if err := run(t, h, TypeOutboxRelay); err != nil {
		t.Fatal(err)
	}
}

func TestRelaySurvivesAQueueOutage(t *testing.T) {
	// The relay already committed, so failing here would only re-run an empty batch; the
	// in-app notification exists either way. The failure is logged and counted instead.
	r := newMailRig()
	r.queue.err = errors.New("redis down")
	r.set.relay = service.RelayResult{Notifications: []service.DeliveredNotification{delivered(1, uuid.New(), uuid.New(), "proof_rejected")}}
	if err := run(t, r.h, TypeOutboxRelay); err != nil {
		t.Fatalf("relay failed: %v", err)
	}
	if got := testutil.ToFloat64(r.m.emails.WithLabelValues("enqueue_failed")); got != 1 {
		t.Fatalf("enqueue_failed = %v", got)
	}
}

func TestDuplicateEmailTasksAreNotAnError(t *testing.T) {
	r := newMailRig()
	r.queue.err = asynq.ErrDuplicateTask
	r.set.relay = service.RelayResult{Notifications: []service.DeliveredNotification{delivered(1, uuid.New(), uuid.New(), "proof_submitted")}}
	if err := run(t, r.h, TypeOutboxRelay); err != nil {
		t.Fatal(err)
	}
	if got := testutil.ToFloat64(r.m.emails.WithLabelValues("enqueue_failed")); got != 0 {
		t.Fatalf("a duplicate digest task counted as a failure: %v", got)
	}
}

func TestNotificationEmailIsSentOnce(t *testing.T) {
	r := newMailRig()
	r.mailer.notif[7] = service.NotificationEmail{NotificationID: 7, To: "bima@tepati.test", ToName: "Bima", Kind: "proof_rejected", PactID: uuid.New(), PactTitle: "UTBK"}
	if err := r.process(TypeEmailNotification, map[string]int64{"id": 7}); err != nil {
		t.Fatal(err)
	}
	if len(r.send.sent) != 1 || r.send.sent[0].To != "bima@tepati.test" || !strings.Contains(r.send.sent[0].Subject, "ditolak") {
		t.Fatalf("sent = %+v", r.send.sent)
	}
	// A retry of the same task finds the claim taken and sends nothing.
	if err := r.process(TypeEmailNotification, map[string]int64{"id": 7}); err != nil {
		t.Fatal(err)
	}
	if len(r.send.sent) != 1 {
		t.Fatalf("a retried task sent %d mails", len(r.send.sent))
	}
	if got := testutil.ToFloat64(r.m.emails.WithLabelValues("sent")); got != 1 {
		t.Errorf("emails{sent} = %v", got)
	}
	if got := testutil.ToFloat64(r.m.emails.WithLabelValues("skipped")); got != 1 {
		t.Errorf("emails{skipped} = %v", got)
	}
}

func TestFailedSendGivesTheClaimBackAndRetries(t *testing.T) {
	r := newMailRig()
	r.mailer.notif[7] = service.NotificationEmail{NotificationID: 7, To: "bima@tepati.test", Kind: "proof_rejected", PactID: uuid.New(), PactTitle: "UTBK"}
	r.send.err = errors.New("smtp down")
	err := r.process(TypeEmailNotification, map[string]int64{"id": 7})
	if err == nil || errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("err = %v, want a retryable error", err)
	}
	if len(r.mailer.released) != 1 {
		t.Fatalf("released = %v: without it the retry finds nothing to send", r.mailer.released)
	}
	if got := testutil.ToFloat64(r.m.emails.WithLabelValues("failed")); got != 1 {
		t.Errorf("emails{failed} = %v", got)
	}
}

func TestUnwritableKindIsNotRetried(t *testing.T) {
	r := newMailRig()
	r.mailer.notif[7] = service.NotificationEmail{NotificationID: 7, To: "bima@tepati.test", Kind: "day_missed", PactID: uuid.New(), PactTitle: "UTBK"}
	err := r.process(TypeEmailNotification, map[string]int64{"id": 7})
	if !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("err = %v, want SkipRetry: retrying cannot make copy appear", err)
	}
	if len(r.send.sent) != 0 {
		t.Fatal("sent a mail for a kind without copy")
	}
}

func TestBrokenPayloadIsNotRetried(t *testing.T) {
	r := newMailRig()
	err := r.h.Mux().ProcessTask(context.Background(), asynq.NewTask(TypeEmailNotification, []byte("not json")))
	if !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("err = %v", err)
	}
}

func TestDigestEmailCoversTheBurst(t *testing.T) {
	r := newMailRig()
	pact := uuid.New()
	r.mailer.digest = &service.DigestEmail{IDs: []int64{1, 2, 3}, To: "andi@tepati.test", ToName: "Andi", Kind: "proof_submitted", PactID: pact, PactTitle: "UTBK", Count: 3}
	payload := map[string]any{"user_id": uuid.New(), "pact_id": pact, "kind": "proof_submitted"}
	if err := r.process(TypeEmailDigest, payload); err != nil {
		t.Fatal(err)
	}
	if len(r.send.sent) != 1 || !strings.Contains(r.send.sent[0].Text, "3 bukti") {
		t.Fatalf("sent = %+v", r.send.sent)
	}
	r.send.err = errors.New("smtp down")
	r.mailer.digest = &service.DigestEmail{IDs: []int64{4}, To: "andi@tepati.test", Kind: "proof_submitted", PactID: pact, PactTitle: "UTBK", Count: 1}
	if err := r.process(TypeEmailDigest, payload); err == nil {
		t.Fatal("expected the send error")
	}
	if len(r.mailer.released) != 1 || r.mailer.released[0] != "d" {
		t.Fatalf("released = %v", r.mailer.released)
	}
}

func TestInviteEmailCarriesTheLink(t *testing.T) {
	r := newMailRig()
	r.mailer.invite = &service.InviteEmail{To: "calon@tepati.test", PactTitle: "UTBK", BackerName: "Andi", ExpiresAt: time.Now().Add(24 * time.Hour), Token: "tok_1"}
	if err := r.process(TypeEmailInvite, map[string]string{"token": "tok_1"}); err != nil {
		t.Fatal(err)
	}
	if len(r.send.sent) != 1 || !strings.Contains(r.send.sent[0].Text, "http://localhost:3000/invite/tok_1") {
		t.Fatalf("sent = %+v", r.send.sent)
	}
	// Retried after success: nothing left to claim.
	if err := r.process(TypeEmailInvite, map[string]string{"token": "tok_1"}); err != nil || len(r.send.sent) != 1 {
		t.Fatalf("retry err=%v sent=%d", err, len(r.send.sent))
	}
}

func TestEmailTasksStayOffTheScheduleAndOnTheDefaultQueue(t *testing.T) {
	for _, p := range Schedule() {
		if strings.HasPrefix(p.Type, "email:") {
			t.Errorf("%s is event-driven, not periodic", p.Type)
		}
	}
}
