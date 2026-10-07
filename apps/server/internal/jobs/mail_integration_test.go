//go:build integration

package jobs

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/andi-frame/lockedin/apps/server/internal/domain"
	"github.com/andi-frame/lockedin/apps/server/internal/notify"
	"github.com/andi-frame/lockedin/apps/server/internal/service"
	"github.com/andi-frame/lockedin/apps/server/internal/store"
	"github.com/andi-frame/lockedin/apps/server/internal/testdb"
)

// inbox is a Sender that keeps what the worker mailed.
type inbox struct {
	mu   sync.Mutex
	mail []notify.Message
}

func (i *inbox) Send(_ context.Context, m notify.Message) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.mail = append(i.mail, m)
	return nil
}

func (i *inbox) to(addr string) []notify.Message {
	i.mu.Lock()
	defer i.mu.Unlock()
	var out []notify.Message
	for _, m := range i.mail {
		if m.To == addr {
			out = append(out, m)
		}
	}
	return out
}

// PLAN 3.2 in miniature: a backer proposes with an invitee address and the worker, with
// nothing but its schedule, mails the invite link; a later signature mails the other member.
func TestWorkerMailsTheInviteAndLaterEvents(t *testing.T) {
	st := testdb.New(t)
	rdb := testdb.RedisIn(t, testRedisDB)
	clock := domain.NewFakeClock(at(1, 0, 0).Add(-7 * 24 * time.Hour))
	svc := service.New(st, clock)
	ctx := context.Background()

	mk := func(email, name string) store.User {
		u, err := st.CreateUser(ctx, store.CreateUserParams{ID: uuid.Must(uuid.NewV7()), Email: email, PasswordHash: "x", DisplayName: name, Locale: "id", Timezone: "Asia/Jakarta"})
		if err != nil {
			t.Fatal(err)
		}
		return u
	}
	backer, doer := mk("andi@tepati.test", "Andi"), mk("bima@tepati.test", "Bima")
	p, err := svc.CreateDraft(ctx, backer.ID, service.DraftInput{Title: "UTBK", Terms: workedTerms(backer.ID)})
	if err != nil {
		t.Fatal(err)
	}
	invitee := "calon@tepati.test"
	token, err := svc.Propose(ctx, backer.ID, p.ID, &invitee)
	if err != nil {
		t.Fatal(err)
	}

	box := &inbox{}
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		done <- Run(runCtx, Options{
			Redis: redisOpt(rdb), Svc: svc, Log: quietLog(), Concurrency: 4,
			Mail: &Mail{Svc: svc, Renderer: notify.NewRenderer("http://localhost:3000"), Sender: box},
			Schedule: []Periodic{
				{"@every 1s", TypeOutboxRelay, QueueDefault, 20 * time.Second},
			},
		})
	}()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(15 * time.Second):
			t.Error("worker did not stop")
		}
	}()

	eventually(t, "the invite email", 20*time.Second, func() bool { return len(box.to(invitee)) > 0 })
	inv := box.to(invitee)[0]
	if !strings.Contains(inv.Text, "http://localhost:3000/invite/"+token) || !strings.Contains(inv.HTML, "/invite/"+token) {
		t.Fatalf("invite mail lacks the link:\n%s", inv.Text)
	}
	if !strings.Contains(inv.Subject, "Andi") {
		t.Fatalf("subject = %q", inv.Subject)
	}

	// A signature notifies the other member by email as well as in the app.
	joined, err := svc.JoinByInvite(ctx, doer.ID, token)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Accept(ctx, backer.ID, joined.ID, joined.TermsHash, "Andi"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the terms_signed email to the doer", 20*time.Second, func() bool { return len(box.to(doer.Email)) > 0 })
	if got := box.to(doer.Email)[0].Subject; !strings.Contains(got, "menandatangani") {
		t.Fatalf("subject = %q", got)
	}

	// The invite is never mailed twice, however often the relay runs.
	time.Sleep(3 * time.Second)
	if n := len(box.to(invitee)); n != 1 {
		t.Fatalf("invitee got %d mails, want 1", n)
	}
	// An address with no account and a join by someone else: the in-app-only kinds stay quiet.
	if n := len(box.to(backer.Email)); n != 0 {
		t.Fatalf("backer got %d mails for in-app-only events: %+v", n, box.to(backer.Email))
	}
}
