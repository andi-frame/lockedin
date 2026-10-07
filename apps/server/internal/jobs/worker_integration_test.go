//go:build integration

package jobs

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"

	"github.com/andi-frame/lockedin/apps/server/internal/domain"
	"github.com/andi-frame/lockedin/apps/server/internal/service"
	"github.com/andi-frame/lockedin/apps/server/internal/store"
	"github.com/andi-frame/lockedin/apps/server/internal/testdb"
)

// Redis logical DB for this package (auth: 15, http: 14).
const testRedisDB = 13

var wib = time.FixedZone("WIB", 7*3600)

func at(day, hour, minute int) time.Time { return time.Date(2026, 11, day, hour, minute, 0, 0, wib) }

func redisOpt(rdb *redis.Client) asynq.RedisClientOpt {
	o := rdb.Options()
	return asynq.RedisClientOpt{Addr: o.Addr, Username: o.Username, Password: o.Password, DB: o.DB}
}

func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// workedTerms is the PLAN worked example; the doer slot is uuid.Nil until they join.
func workedTerms(backer uuid.UUID) domain.Terms {
	cap1500 := int64(1500)
	return domain.Terms{
		Version: 1, Timezone: "Asia/Jakarta",
		StartsOn: domain.MustDate("2026-11-02"), EndsOn: domain.MustDate("2026-11-29"),
		CutoffLocalTime: "23:59", GraceMinutes: 30, CoinRateIDR: 1000,
		InitialPot: 1000, PotFloor: 0, PotCap: &cap1500,
		ReviewWindowHours: 24, DisputeWindowHours: 24, DisputeResolutionHours: 48, OverrideWindowHours: 48,
		MaxOverrides: 3, BackerCommits: true,
		Members: map[uuid.UUID]domain.MemberTerms{
			backer:   {Role: domain.RoleBacker, Commitment: "Belajar Kalkulus 2 jam", Schedule: []int{1, 2, 3, 4, 5}, PenaltyPerMiss: 50, RestDays: 2},
			uuid.Nil: {Role: domain.RoleDoer, Commitment: "Latihan soal UTBK 50 soal", Schedule: []int{1, 2, 3, 4, 5, 6}, PenaltyPerMiss: 50, RestDays: 2},
		},
	}
}

// activePact builds the PLAN worked example through the normal service flow and activates it.
func activePact(t *testing.T, st *store.Store, clock *domain.FakeClock, svc *service.Service) (store.Pact, store.User, store.User) {
	t.Helper()
	ctx := context.Background()
	mk := func(email, name string) store.User {
		u, err := st.CreateUser(ctx, store.CreateUserParams{ID: uuid.Must(uuid.NewV7()), Email: email, PasswordHash: "x", DisplayName: name, Locale: "id", Timezone: "Asia/Jakarta"})
		if err != nil {
			t.Fatal(err)
		}
		return u
	}
	backer, doer := mk("andi@tepati.test", "Andi"), mk("bima@tepati.test", "Bima")
	terms := workedTerms(backer.ID)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	clock.Set(at(1, 0, 0).Add(-7 * 24 * time.Hour))
	p, err := svc.CreateDraft(ctx, backer.ID, service.DraftInput{Title: "UTBK", Terms: terms})
	must(err)
	tok, err := svc.Propose(ctx, backer.ID, p.ID, nil)
	must(err)
	p, err = svc.JoinByInvite(ctx, doer.ID, tok)
	must(err)
	_, err = svc.Accept(ctx, backer.ID, p.ID, p.TermsHash, "Andi")
	must(err)
	_, err = svc.Accept(ctx, doer.ID, p.ID, p.TermsHash, "Bima")
	must(err)
	clock.Set(at(2, 0, 0))
	_, err = svc.ActivateDuePacts(ctx)
	must(err)
	p, err = st.GetPact(ctx, p.ID)
	must(err)
	return p, backer, doer
}

func eventually(t *testing.T, what string, within time.Duration, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timed out after %s waiting for %s", within, what)
}

// PLAN 3.1 in miniature: the worker, with nothing but its schedule, turns an overdue
// check-in into `missed` with a penalty row and a notification, then stops cleanly.
func TestWorkerSettlesAnOverdueCheckInOnItsOwn(t *testing.T) {
	st := testdb.New(t)
	rdb := testdb.RedisIn(t, testRedisDB)
	clock := domain.NewFakeClock(at(2, 0, 0))
	svc := service.New(st, clock)
	pact, _, doer := activePact(t, st, clock, svc)
	clock.Set(at(3, 0, 31)) // 2 Nov is over: cutoff 23:59 plus 30 minutes' grace

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, Options{
			Redis: redisOpt(rdb), Svc: svc, Log: quietLog(), Concurrency: 4,
			Schedule: []Periodic{
				{"@every 1s", TypeSettlementSweep, QueueCritical, 20 * time.Second},
				{"@every 1s", TypeOutboxRelay, QueueDefault, 20 * time.Second},
			},
		})
	}()

	day := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	missed := func() bool {
		rows, err := st.ListCheckInsForPact(context.Background(), store.ListCheckInsForPactParams{PactID: pact.ID, FromDate: day, ToDate: day})
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, r := range rows {
			if r.Status == "missed" {
				n++
			}
		}
		return n == 2 // both members had a check-in that day
	}
	eventually(t, "2 Nov check-ins to become missed", 20*time.Second, missed)

	// The backer commits too, so their miss adds coins while the doer's takes them out and the
	// balance can net to zero. The ledger rows are what prove both penalties landed.
	lines, err := st.ListLedgerPage(context.Background(), store.ListLedgerPageParams{PactID: pact.ID, MaxRows: 100})
	if err != nil {
		t.Fatal(err)
	}
	penalties := 0
	for _, l := range lines {
		if l.Kind != string(domain.LedgerPotInitial) {
			penalties++
		}
	}
	if penalties != 2 {
		t.Fatalf("ledger has %d rows besides the initial pot, want a penalty and a contribution: %+v", penalties, lines)
	}
	eventually(t, "the day_missed notification", 10*time.Second, func() bool {
		rows, _, _, err := svc.ListNotifications(context.Background(), doer.ID, nil, 50, false)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			if r.Kind == "day_missed" {
				return true
			}
		}
		return false
	})

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned %v on a clean shutdown", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("worker did not stop after its context was cancelled")
	}
}

func TestMetricsShowQueueSizesAndTransitions(t *testing.T) {
	rdb := testdb.RedisIn(t, testRedisDB)
	opt := redisOpt(rdb)
	client := asynq.NewClient(opt)
	defer client.Close()
	if _, err := client.Enqueue(asynq.NewTask("media:process", nil), asynq.Queue(QueueMedia)); err != nil {
		t.Fatal(err)
	}

	m := NewMetrics()
	insp := asynq.NewInspector(opt)
	defer insp.Close()
	m.WatchQueues(insp)

	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := rec.Body.String()
	for _, want := range []string{
		`tepati_asynq_queue_tasks{queue="media",state="pending"} 1`,
		`tepati_asynq_queue_tasks{queue="critical",state="pending"} 0`, // a queue that does not exist yet reads as empty
		`tepati_settlement_transitions_total{type="closed"} 0`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("metrics missing %q", want)
		}
	}
}
