# Tepati: project status and handoff

Last updated: 2026-10-07, at the end of Phase 3. This is the first thing to read when you start a new session. `docs/PLAN.md` says *what* is next; this file says *where we are*, what exists, what was decided on the way, and what to watch out for. Update it at the end of every phase.

## 1. Where we are

| Phase | State | Notes |
|---|---|---|
| 0 Repository foundation | done | Bun workspace, env scripts, infra compose, dev orchestrator |
| 1 Backend core (Go) | done | Schema, pure domain, store, pact and check-in services, settlement sweep, auth |
| 2 API contract and HTTP | done | OpenAPI contract, Fiber app, all handlers (uploads stubbed, see §6) |
| 3 Worker | done | 3.1 asynq worker, schedule, outbox relay, reminders, `tepatictl seed`/`pact show`, air files. 3.2 notifications and email (Mailpit) |
| **4 Uploads and media** | **next** | 4.1 BlobStore drivers, 4.2 upload intent, complete and processing |
| 5–9 | not started | Web app, Docker, quality gates, staging |

The first unchecked task in `docs/PLAN.md` is **4.1**.

## 2. Branch and merge state

Nothing has been pushed or merged. `main` is still at `e4f987f` (end of Phase 1). Everything since lives on **stacked** branches, each cut from the one before:

```
main ─ p2.1-openapi ─ p2.2-fiber-middleware ─ p2.3-handlers ─ p3.1-worker-jobs ─ p3.2-notifications-email   (HEAD)
```

- Merge in that order, or merge `p3.2-notifications-email` alone since it contains all the others.
- Branch for 4.1: cut `p4.1-blobstore` from `p3.2-notifications-email` if it is still unmerged, otherwise from `main`.
- `p2.3` is far over the ~600-line PR guideline in `AGENTS.md`. It is split into three commits (contract fixes, service layer, handlers) so it can be reviewed commit by commit. `p3.1` is four commits (service relay and reminders, worker, CLI and air, docs). `p3.2` is service claims and the invite event, the `notify` package, the worker email tasks, a copy fix, and docs.
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

Last result (2026-10-07, end of 3.1): all of the above green; `internal/domain` coverage 99.5%.

Integration tests create a throwaway database per test (`tepati_test_<random>`, dropped afterwards) and use Redis logical DBs: **auth uses 15, http uses 14, jobs uses 13**. Packages run in parallel and each flushes its DB, so **a new package that uses Redis in tests must take its own index** (`testdb.RedisIn(t, n)`; next free is 12).

Run it all: `bun run db:migrate`, then `bun run dev:hybrid` (infra, then the API and the worker through air; no web app yet). In a second shell, `bun run db:seed` creates an active pact with overdue check-ins, and `bun run ctl -- pact show <id>` shows the worker marking them `missed` within about a minute. Worker metrics: `curl localhost:9091/metrics`. Stop `dev:hybrid` with Ctrl+C (when it runs in a background shell, kill the `air` and `tmp/*.exe` processes). The Go toolchain `PATH` fix above must be in the shell that starts `dev:hybrid`, because air runs `go build`.

## 4. What exists

### Backend (`apps/server`, Go module `github.com/andi-frame/lockedin/apps/server`)

| Package | Role |
|---|---|
| `internal/config` | env parsing and validation (`caarlos0/env`), lists every missing var at once |
| `internal/domain` | pure rules, no I/O: `Terms` (validate, canonical hash), deadlines per timezone, the check-in FSM `Transition(ci, ev, now, ctx)` returning effects, ledger clamp math, proof-doc allow-list, `Clock` (`SystemClock`, `FakeClock`), `ValidReason` |
| `internal/store` | sqlc-generated queries and `WithTx`. Queries live in `db/queries/*.sql`, migration in `db/migrations/` |
| `internal/service` | every state change, one transaction each. `pacts.go` (lifecycle), `checkins.go` (transitions, `SweepDeadlines`, `ClosePacts`), `payouts.go` (mark paid, confirm), `outbox.go` (`RelayOutbox`), `reminders.go` (`SendReminders`), `mailing.go` (`Claim*Email`/`Release*Email`), `read.go` (membership-filtered views, keyset pages, `my_actions`) |
| `internal/auth` | argon2id, Redis sessions (hashed token), CSRF double-submit, login limiter, `RequireUser`, `UserFromContext` |
| `internal/http` | the Fiber app. `server.go` (middleware stack), `problem.go` + `statuses.go` (errors), `idempotency.go`, `ratelimit.go`, `observe.go` (access log + Prometheus), `health.go`, `handlers*.go` + `mappers.go` + `cursor.go` (the strict-server implementation) |
| `internal/http/api` | **generated** by oapi-codegen from `api/openapi.yaml`. Never edit |
| `internal/jobs` | the asynq worker. `jobs.go` (task types, `Schedule()` table, `Settlement` interface, handlers), `mail.go` (email tasks and `Mail` dependencies), `worker.go` (`Run`: server + scheduler + metrics listener, graceful stop), `metrics.go` (per-instance Prometheus registry and the asynq queue collector) |
| `internal/notify` | email, no database: `delivery.go` (which kinds are mailed, SPEC §9), `render.go` (Indonesian copy, `html/template` plus plain-text layouts in `templates/`), `smtp.go` (`SMTP` sender: STARTTLS when offered, context deadlines, header-injection checks) |
| `internal/ctl` | logic of `tepatictl`: `SeedOverdue`, `SeedInvite`, `Show` (integration-tested) |
| `internal/testdb` | per-test database and Redis helpers (integration tag) |
| `cmd/api` | wired and working: Postgres, Redis, services, handlers, readiness checks, graceful shutdown |
| `cmd/worker` | wired and working: store, service with `SystemClock`, SMTP sender (`SMTP_URL`, `MAIL_FROM`), `APP_BASE_URL` for links, `jobs.Run`. `WORKER_CONCURRENCY`, `WORKER_METRICS_PORT` |
| `cmd/tepatictl` | `version`, `seed --scenario overdue\|invite`, `pact show <id>` |

### Contract and web types
`api/openapi.yaml` (OpenAPI 3.1, 31 operations plus health). `bun run codegen` regenerates sqlc, the Go strict server, and `apps/web/src/lib/api/schema.d.ts`. `apps/web` has no app yet, only that generated file.

### Scripts
`bun run codegen|lint|test|setup|infra:*|garage:init|s3:smoke|db:*|ctl` work, `db:seed` included, and `dev:hybrid` starts the api and worker (`apps/server/.air.*.toml`; `scripts/lib/air.ts` adds polling when `AIR_POLL=1`). Still stubs that exit non-zero via `scripts/todo.ts`: `test:e2e` (→ 5.3), `deploy:*` (→ 7.x).

## 5. Decisions and rulings made so far

Why things are the way they are, beyond the ADRs. API conventions are written up in `docs/adr/0010-api-conventions.md`.

**Phase 1**
- The worked example in `PLAN.md` is a real integration test (`TestWorkedExample`) and ends at balance 800 before payout.
- Penalties apply only when a check-in becomes final, never on the first rejection; an upheld dispute moves no coins (a `reversal` row exists only for a penalised check-in that later becomes approved).

**Phase 3.2**
- **Email is event-driven, not scheduled.** After `RelayOutbox` commits, the `outbox:relay` handler enqueues `email:notification` (one per Immediate kind), `email:digest` (proof submitted) and `email:invite` on the `default` queue. The service returns `RelayResult{Notifications (with ids), Invites, Skipped}` for this. If the enqueue fails (Redis down) it is logged and counted in `tepati_emails_total{result="enqueue_failed"}`, not retried: the in-app notification exists, only the email is lost.
- **Claim, send, release.** `Service.Claim*Email` sets `emailed_at` in the statement that reads the row, so a retried or duplicated task finds nothing to send. A failed send calls `Release*Email` so the asynq retry (5 attempts, backoff) can send. A crash between claim and send loses one email, never doubles one.
- **The invite token and the outbox.** `Propose` with an address writes outbox topic `invite_mail` with `{pact_id, email, token}`. The relay hands it to the email task and `MarkOutboxDispatched` runs `payload - 'token'` in the same statement, so the plaintext is in Postgres only until the next relay pass (seconds). The token also sits in the asynq task payload (Redis) until the email is sent; the task has `Retention(0)` so finished tasks leave no copy. A link-only invite (no address) writes nothing.
- **Digest.** `email:digest` is `Unique(window+timeout)` and `ProcessIn(5 min)`, so a burst of `proof_submitted` events becomes one email per (user, pact). A submission that lands while that digest is being sent waits for the next one (it still shows in-app).
- **Which kinds are mailed** is `notify.delivery` in `internal/notify/delivery.go`, matching the SPEC §9 table: terms changed/signed, proof submitted (digest), rejected, overridden, auto-approved, dispute opened, pact settled. Adding a kind there without copy in `render.go` fails `TestEveryEmailedKindHasCopy`.
- **Links** use `APP_BASE_URL`: `/pacts/<id>`, `/review`, `/invite/<token>`. Phase 5/6 must serve those routes.
- **Recipient name** falls back to "teman" (invitees have no account).

**Phase 3.1**
- **The schedule** is the table in `jobs.Schedule()`: sweep and activate every 1 min and close every 5 min on `critical`; outbox relay every 5 s and reminders every 5 min on `default`; `uploads:gc` hourly on `media`. Periodic tasks run with `MaxRetry(0)` and a unique lock just under their period (two replicas enqueue once). The next tick is the retry, because every job re-derives its work from the database.
- **A sweep drains its backlog**: it repeats while a batch (200) comes back full, up to 10 passes per run, so downtime is worked off in one go.
- **Outbox relay** (`service.RelayOutbox`): fetch pending rows with `SKIP LOCKED`, insert the `notifications` rows and mark dispatched in one transaction. A row with an unknown topic or an unreadable payload is dispatched and counted as skipped, so it can never block the queue. It returns the delivered notifications so 3.2 can enqueue emails after the commit.
- **Reminders** (`service.SendReminders`, SPEC §9): 3 h reminder when the cutoff is within (30 min, 3 h], 30 min reminder within (0, 30 min], both only for `open` check-ins of `active` pacts, so a check-in first seen late gets just the urgent one. The reviewer gets `review_deadline_soon` 2 h before a review deadline. Dedupe is the new `reminders_sent` table (migration `20261008000001`), inserted in the same transaction as the outbox row.
- **Settlement metrics count by job type** (`deadline` = check-ins moved by a passed deadline, `activated`, `closed`), not by FSM status, because `SweepDeadlines` returns only a count. If per-status counts are wanted, make the service return the transitions.
- **Seeds go through the service** with a `FakeClock` set in the past (invariant 3), so a seeded pact is indistinguishable from a real one. Seed users are reused across runs; each run adds a new pact.
- **asynq's `GetQueueInfo` does not wrap `ErrQueueNotFound`** for a queue that was never created (only `DeleteQueue` does); the collector lists queues first. A test guards this.

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
- Email: no `sent`/bounce tracking beyond `emailed_at`, no unsubscribe or per-user preferences (not in SPEC), and the `en` locale is not rendered (every email is Indonesian; `users.locale` is read but unused). Email copy lives in Go (`internal/notify/render.go`), not `messages/id.json`.
- Production SMTP (TLS, auth) is untested against a real provider; only Mailpit and a fake server were used. `smtps://` and `user:pass@` are implemented.
- The invite email is the only place the plaintext token is sent. If the email is lost (enqueue failure, crash after claim), the backer still has the link from the `Propose` response; there is no "resend invite" endpoint.
- `uploads:gc` is a no-op until the BlobStore exists (4.1/4.2). The `media` concurrency cap (2) must be a semaphore in the `media:process` handler (4.2).
- The worker has `/metrics` and `/healthz` only, no `/readyz`. Fine for now; add one with the compose healthchecks (7.1).
- `tepati_settlement_transitions_total` is by job type, see §5.
- No test-clock endpoint yet (`CLOCK_OVERRIDE`); the e2e work (5.3/6.x) needs it. The API and worker use `domain.SystemClock`.
- context7 MCP was still not authenticated; asynq and Prometheus APIs were checked by reading module source under `~/go/pkg/mod`. If you can, authenticate it before 4.1 (aws-sdk-go-v2 and Garage).

## 7. Brief for Phase 4 (uploads and media)

**Read first:** `docs/PLAN.md` tasks 4.1 and 4.2, `docs/ARCHITECTURE.md §6` (upload pipeline), `docs/SPEC.md §8`, `docs/adr/0005` (storage), and `internal/jobs/jobs.go` for how a task type is registered.

**Already in place:**
- Garage is in the infra compose and `bun run garage:init` plus `bun run s3:smoke` exist (Phase 0). `UPLOAD_MODE`, the S3 and `STORAGE_*` variables are in config and `deploy/env/.env.example`.
- The three upload operations (`createUpload`, `completeUpload`, `getAttachment`) are routed and answer `503 server.unavailable` until 4.1/4.2 fill them in. `attachments` table exists; `Attachment.urls` is never filled.
- The `media` queue exists with weight 1; `uploads:gc` is scheduled hourly and does nothing. The `media` concurrency cap (2) must be a semaphore inside the `media:process` handler because asynq has no per-queue concurrency.
- Handlers take a small interface and a `Mail`-style dependency struct (see `jobs/mail.go`); copy that shape for the media handler so it can be faked in unit tests.

**To build:** 4.1 the `storage.BlobStore` interface with `s3` and `fs` drivers and `TestPresignedPutRejectsWrongLength` against Garage (record the result in ADR-0005; if Garage does not enforce the signed length, `UPLOAD_MODE=proxy` becomes the default). 4.2 the endpoints and the `media:process` handler with golden tests under `internal/media/testdata` (EXIF stripped, wrong type rejected, 200 s video rejected, 1080p to 720p). ffmpeg and vips are called through `exec.CommandContext` with arg slices and timeouts.

**Process reminders (from `AGENTS.md`):** one PLAN task per branch (`p4.1-blobstore`), test first, update `docs/PLAN.md` with the commit hash, update this file and `ARCHITECTURE` when behaviour changes, and refresh the knowledge graph with `/graphify . --update` after large changes (it was last refreshed after Phase 2). Do not start 4.2 in the same session as 4.1 unless asked.
