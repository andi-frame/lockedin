# Tepati: project status and handoff

Last updated: 2026-10-07, at the end of Phase 2. This is the first thing to read when you start a new session. `docs/PLAN.md` says *what* is next; this file says *where we are*, what exists, what was decided on the way, and what to watch out for. Update it at the end of every phase.

## 1. Where we are

| Phase | State | Notes |
|---|---|---|
| 0 Repository foundation | done | Bun workspace, env scripts, infra compose, dev orchestrator |
| 1 Backend core (Go) | done | Schema, pure domain, store, pact and check-in services, settlement sweep, auth |
| 2 API contract and HTTP | done | OpenAPI contract, Fiber app, all handlers (uploads stubbed, see §6) |
| **3 Worker** | **next** | 3.1 asynq server, scheduler, outbox relay; 3.2 notifications and email |
| 4–9 | not started | Uploads, web app, Docker, quality gates, staging |

The first unchecked task in `docs/PLAN.md` is **3.1**.

## 2. Branch and merge state

Nothing has been pushed or merged. `main` is still at `e4f987f` (end of Phase 1). Phase 2 lives on three **stacked** branches, each cut from the one before:

```
main ─ p2.1-openapi ─ p2.2-fiber-middleware ─ p2.3-handlers   (HEAD, d03fe37)
```

- Merge in that order, or merge `p2.3-handlers` alone since it contains the other two.
- Branch for Phase 3: cut `p3.1-worker-jobs` from `p2.3-handlers` if it is still unmerged, otherwise from `main`.
- `p2.3` is far over the ~600-line PR guideline in `AGENTS.md`. It is split into three commits (contract fixes, service layer, handlers) so it can be reviewed commit by commit.
- Commit messages carry no Claude attribution lines (the project owner's rule).

## 3. How to verify the current state

Prerequisite on this Windows machine: Go auto-switching is broken (installed Go 1.25.3, `go.mod` needs 1.26.0). Before any Go command, `bun run lint`, `bun run test`, or `bun run codegen`, run in the shell:

```bash
export PATH="/c/Users/andif/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.0.windows-amd64/bin:$PATH" GOTOOLCHAIN=local
```

Infra must be up (`bun run infra:up`): Postgres, Redis, Garage, Mailpit. Then:

```bash
bun run lint                                                  # scripts tsc, OpenAPI lint, gofmt, go vet
bun run test                                                  # bun script tests + Go unit tests (no DB needed)
bun run codegen -- --check                                    # generated code is fresh (run after committing)
cd apps/server && go test -race -count=1 -tags=integration ./...   # everything, real Postgres + Redis
```

Last result (2026-10-07): all of the above green; `internal/domain` coverage 99.5%.

Integration tests create a throwaway database per test (`tepati_test_<random>`, dropped afterwards) and use Redis logical DBs: **auth uses 15, http uses 14**. Packages run in parallel and each flushes its DB, so **a new package that uses Redis in tests must take its own index** (`testdb.RedisIn(t, n)`; next free is 13).

To run the API by hand: `cd apps/server && go build -o /tmp/tepati-api.exe ./cmd/api`, then load `.env` into the shell and run it (set `API_PORT` if 8080 is busy). `bun run dev:hybrid` does **not** start the api or worker yet: `scripts/dev.ts` only starts a process when `apps/server/.air.<name>.toml` exists, and neither file exists. `air` is installed (`~/go/bin/air`). Creating the two `.air.*.toml` files belongs to the dev-loop work (PLAN 3.1 verify uses `dev:hybrid`, so add `.air.worker.toml` and `.air.api.toml` there).

## 4. What exists

### Backend (`apps/server`, Go module `github.com/andi-frame/lockedin/apps/server`)

| Package | Role |
|---|---|
| `internal/config` | env parsing and validation (`caarlos0/env`), lists every missing var at once |
| `internal/domain` | pure rules, no I/O: `Terms` (validate, canonical hash), deadlines per timezone, the check-in FSM `Transition(ci, ev, now, ctx)` returning effects, ledger clamp math, proof-doc allow-list, `Clock` (`SystemClock`, `FakeClock`), `ValidReason` |
| `internal/store` | sqlc-generated queries and `WithTx`. Queries live in `db/queries/*.sql`, migration in `db/migrations/` |
| `internal/service` | every state change, one transaction each. `pacts.go` (lifecycle), `checkins.go` (transitions, `SweepDeadlines`, `ClosePacts`), `payouts.go` (mark paid, confirm), `read.go` (membership-filtered views, keyset pages, `my_actions`) |
| `internal/auth` | argon2id, Redis sessions (hashed token), CSRF double-submit, login limiter, `RequireUser`, `UserFromContext` |
| `internal/http` | the Fiber app. `server.go` (middleware stack), `problem.go` + `statuses.go` (errors), `idempotency.go`, `ratelimit.go`, `observe.go` (access log + Prometheus), `health.go`, `handlers*.go` + `mappers.go` + `cursor.go` (the strict-server implementation) |
| `internal/http/api` | **generated** by oapi-codegen from `api/openapi.yaml`. Never edit |
| `internal/testdb` | per-test database and Redis helpers (integration tag) |
| `cmd/api` | wired and working: Postgres, Redis, services, handlers, readiness checks, graceful shutdown |
| `cmd/worker` | **skeleton only**: logs and waits for SIGTERM. Phase 3 fills it |
| `cmd/tepatictl` | **only `version`**. Phase 3 adds `seed` and `pact show` |

### Contract and web types
`api/openapi.yaml` (OpenAPI 3.1, 31 operations plus health). `bun run codegen` regenerates sqlc, the Go strict server, and `apps/web/src/lib/api/schema.d.ts`. `apps/web` has no app yet, only that generated file.

### Scripts
`bun run codegen|lint|test|setup|infra:*|garage:init|s3:smoke|db:*` work. Still stubs that exit non-zero via `scripts/todo.ts`: `db:seed` (→ 3.1), `test:e2e` (→ 5.3), `deploy:*` (→ 7.x).

## 5. Decisions and rulings made so far

Why things are the way they are, beyond the ADRs. API conventions are written up in `docs/adr/0010-api-conventions.md`.

**Phase 1**
- The worked example in `PLAN.md` is a real integration test (`TestWorkedExample`) and ends at balance 800 before payout.
- Penalties apply only when a check-in becomes final, never on the first rejection; an upheld dispute moves no coins (a `reversal` row exists only for a penalised check-in that later becomes approved).

**Phase 2**
- **Join endpoint added.** `POST /invites/{token}/join` did not exist in `ARCHITECTURE §5`, but an invitee must become a member before they can accept. Joining re-keys the doer slot in the terms, which changes `terms_hash` and clears signatures, so the invitee then accepts the new hash.
- **Uploads stubbed.** `createUpload`, `completeUpload`, `getAttachment` are routed but answer `503 server.unavailable` until PLAN 4.1/4.2. So 28 of 31 operations are implemented.
- **Dispute resolution needs a reason for both outcomes** (SPEC §2). The domain only enforces it for dismiss, so the handler enforces it for uphold via `domain.ValidReason`. If SPEC changes, change the handler.
- **New error codes**: `pact.not_doer`, `request.too_large`, `request.unsupported_media_type`, `method_not_allowed`. Decision actions `submit` and `finalize` were added to the contract enum because the database already allows them.
- **Editing terms**: either member may edit while `proposed` (SPEC §3), so `PATCH /pacts/{id}` has no backer-only check. On a running pact it is `409 pact.invalid_state`.
- **Request bodies**: JSON only; an empty body on POST/PUT/PATCH counts as `{}` (the generated handlers bind a body even when the contract calls it optional and fail on an empty one); any other content type is `415`.
- **Access log sits outside `recover`** (deliberate deviation from the order in the first draft of `ARCHITECTURE §3`) so panics are logged as the 500 they became.
- **Public routes** (register, login, invite preview) are listed in code (`isPublic` in `server.go`), because the Fiber v3 generator does not emit per-route security scopes. `spec_test.go` ties that list to the `security: []` operations in the contract, so it cannot drift.
- **`ctx.Context()` in Fiber v3 is a plain context**, not the fasthttp context. `auth.RequireUser` therefore stores the user with `c.SetContext`, and strict handlers read it with `auth.UserFromContext`. Client IP is stored the same way (`ClientIPFromContext`).
- **Fiber `c.IP()` is empty** when a trusted-proxy header is configured but the request has none. Always use `clientIP(c)` in `internal/http` (it falls back to the peer address); a test guards this.
- **`my_actions` is computed by dry-running `domain.Transition`** for each action, so the web app never re-derives rules.
- **Cookies**: `Secure` everywhere except `APP_ENV=dev`.
- **Handler tests carry the `integration` tag** (they need Postgres and Redis). The untagged `go test ./internal/http/...` covers the middleware with miniredis.

## 6. Known gaps and deferred work

- Upload endpoints (above). `Attachment.urls` is never filled until the BlobStore exists.
- `problem.errors[]` (per-field detail) is in the contract but never populated; validation failures carry a text `detail` only. Phase 6.1 (the wizard) will want structured errors from `Terms.Validate`.
- `Service.Today` loads all of a user's pacts and filters `active|settling` in Go. Fine now; add a status filter to the query once completed pacts pile up.
- Server-level rejects (for example an oversize body) are handled by Fiber below the middleware, so they are not access-logged.
- Idempotency fails open if Redis is down (logged); the DB's own idempotency keys and status guards still protect money.
- Notifications are **read** by the API but nothing writes the `notifications` table yet (the outbox relay is 3.1/3.2).
- No test-clock endpoint yet (`CLOCK_OVERRIDE`); the e2e work (5.3/6.x) needs it. The API and worker use `domain.SystemClock`.
- context7 MCP was not authenticated in the last session; library APIs were checked by reading module source under `~/go/pkg/mod`. If you can, authenticate it before using asynq.

## 7. Brief for Phase 3

**Read first:** `docs/PLAN.md` tasks 3.1 and 3.2, `docs/SPEC.md §7 and §9`, `docs/adr/0004-queue-asynq.md`, `ARCHITECTURE §3` (queues and the outbox rule).

**Already built and reusable (call these from job handlers, never re-implement):**
- `service.SweepDeadlines(ctx, batch) (int, error)`: SPEC §7 steps 1–6, safe to run concurrently.
- `service.ActivateDuePacts(ctx) ([]uuid.UUID, error)`: scheduled → active.
- `service.ClosePacts(ctx, batch) ([]uuid.UUID, error)`: active → settling with the payout entry.
- All three use the injected `domain.Clock`; the worker passes `domain.SystemClock{}`. Build the service with `service.New(store.NewStore(pool), clock)` as `cmd/api/main.go` does.
- `app.Main(name, fn)` gives you config, logger and a context cancelled on SIGINT/SIGTERM.

**The outbox, which the relay must drain:**
- Every transition inserts a row into `outbox` in the same transaction (`topic = "notify"`, payload is `service.Notification` JSON: `user_id`, `kind`, `pact_id`, optional `check_in_id`).
- Queries: `FetchPendingOutbox(limit)` uses `FOR UPDATE SKIP LOCKED`, so call it inside `store.WithTx` together with `MarkOutboxDispatched` and the `InsertNotification` rows, in one transaction. Enqueue emails only after commit (invariant: never enqueue inside a transaction that might roll back).
- Kinds emitted today: `member_joined`, `terms_changed`, `terms_signed`, `pact_scheduled`, `pact_settled`, `proof_submitted`, `proof_edited`, `proof_approved`, `proof_rejected`, `proof_auto_approved`, `proof_overridden`, `rejection_final`, `rest_declared`, `day_missed`, `dispute_opened`, `dispute_upheld`, `dispute_dismissed`, `payout_marked_paid`, `payout_confirmed`.
- In the contract's `NotificationKind` but **not emitted yet**: `reminder_cutoff_3h`, `reminder_cutoff_30m`, `review_deadline_soon` (the `reminders:cutoff` job produces them). The `notifications` table has no uniqueness, so the reminder job must dedupe itself (asynq unique tasks plus an idempotent insert) because it runs every 5 minutes.
- `GET /notifications` returns `payload` as an object holding `pact_id` and, where relevant, `check_in_id`; store the `Notification` JSON as the row payload.

**To build:**
- Add `github.com/hibiken/asynq` (not in `go.mod` yet). Queues `critical/default/media` weights 6/3/1, periodic tasks as listed in 3.1, graceful shutdown. Give the new package its own Redis test DB index (next free: 13). Remember that asynq also needs Redis, so check what DB `REDIS_URL` selects.
- `tepatictl seed --scenario overdue|invite` and `tepatictl pact show <id>`, then point `bun run db:seed` at it (currently `scripts/todo.ts`). `Terms.Validate` does **not** reject past dates, and the services take an injected clock, so an overdue scenario can be built by running the normal service flow with a `FakeClock` set in the past; the real worker then sweeps it with the system clock.
- `.air.worker.toml` and `.air.api.toml` in `apps/server`, so `bun run dev:hybrid` starts them (3.1's verify step uses it). Use polling when `AIR_POLL=1` (RUNNING §4).
- Worker `/metrics` per `ARCHITECTURE §10`: queue sizes, settlement transitions by type. `internal/http/observe.go` shows the per-instance Prometheus registry pattern.
- 3.2: SMTP via Mailpit in dev (`SMTP_URL`, `MAIL_FROM` already in config), Indonesian `html/template` plus plain-text fallback, triggers from SPEC §9. Notification copy for in-app lives in the web app's `messages/id.json` (Phase 5/6), so the API only stores kinds and ids.

**Process reminders (from `AGENTS.md`):** one PLAN task per branch (`p3.1-…`, `p3.2-…`), test first (money, time and state code is 🔒 and needs table-driven unit tests plus an integration test), update `docs/PLAN.md` with the commit hash, update this file and `ARCHITECTURE` when behaviour changes, and refresh the knowledge graph with `/graphify . --update` after large changes.
