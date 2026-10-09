# Tepati: project status and handoff

Last updated: 2026-10-08, after task 8.4 (the load test; the backend is complete, Phases 5, 6 and 7 are done and Phase 8 is in progress). This is the first thing to read when you start a new session, together with `docs/HANDOVER.md` (how to work in this repo and on this machine, whatever agent you are). `docs/PLAN.md` says *what* is next; this file says *where we are*, what exists, what was decided on the way, and what to watch out for. Update it at the end of every phase.

## 1. Where we are

| Phase | State | Notes |
|---|---|---|
| 0 Repository foundation | done | Bun workspace, env scripts, infra compose, dev orchestrator |
| 1 Backend core (Go) | done | Schema, pure domain, store, pact and check-in services, settlement sweep, auth |
| 2 API contract and HTTP | done | OpenAPI contract, Fiber app, all 31 operations |
| 3 Worker | done | 3.1 asynq worker, schedule, outbox relay, reminders, `tepatictl seed`/`pact show`, air files. 3.2 notifications and email (Mailpit) |
| 4 Uploads and media | done | 4.1 BlobStore drivers (s3 and fs, Garage enforces the signed length). 4.2 upload endpoints, `media:process`, `uploads:gc` |
| 5 Web foundation | done | 5.1 Next.js app on Bun (PR #3). 5.2 design tokens and primitives (PR #4). 5.3 auth pages, app shell, auth guard and the Playwright e2e (PR #5) |
| 6 Web features | done | 6.1 new pact wizard and invite/accept flow (PR #6). 6.2 Today screen (PR #7). 6.3 proof editor and submission (PR #8). 6.4 pact page with the coin book and calendar (PR #9). 6.5 review queue and check-in detail (PR #10). 6.6 settlement and payout screens (PR #11). 6.7 notifications inbox and settings: done on `p6.7-notifications` (not pushed or merged yet). Next: Phase 7 |
| 7 Docker and deploy | done | 7.1 Dockerfiles (PR #13). 7.2 compose dev and prod, Caddy, deploy:up, deploy:scale, backups (PR #14). 7.3 native mode polish: done on `p7.3-native-polish` (not pushed or merged yet) |
| 8 Quality gates | in progress | 8.1 CI (PR #16). 8.2 design finish review, `DESIGN.md` (PR #17). 8.3 web-design-guidelines audit (PR #18). 8.4 k6 load test: done on `p8.4-load-test` (not pushed or merged yet). Next: 8.5 security review |
| 9 | not started | Staging deploy, landing page |

The first unchecked task in `docs/PLAN.md` is **8.5** (the security review). The real staging deploy (9.1) needs the owner's decisions on the SMTP provider, the domain and DNS (a `media.` host too), see `docs/HANDOVER.md` §10.

## 2. Branch and merge state

Phases 2 to 4 are **merged into `main`**: pull request #1 (`p4.2-uploads` -> `main`) was merged on 2026-10-08 with a merge commit (`054e61a`), not squashed, so every commit hash recorded in `docs/PLAN.md` is in `main`'s history. Before the merge the work lived on seven **stacked** branches, each cut from the one before:

```
e4f987f (end of Phase 1) ─ p2.1-openapi ─ p2.2-fiber-middleware ─ p2.3-handlers ─ p3.1-worker-jobs ─ p3.2-notifications-email ─ p4.1-blobstore ─ p4.2-uploads ─ merge 054e61a (main)
```

- The seven task branches (`p2.1-openapi` ... `p4.2-uploads`) were deleted from `origin` and locally on 2026-10-08, at the owner's request, after the merge. Their commits are in `main`.
- 5.1 was merged into `main` as PR #3 (merge commit `f715daf`, 2026-10-08). The branch `p5.1-nextjs-app` still exists on `origin` (the owner has not asked to delete it).
- 5.2 was merged as PR #4 (merge commit `e189035`, 2026-10-08). The branch `p5.2-tokens-primitives` still exists on `origin`, like `p5.1-nextjs-app` (the owner has not asked to delete them).
- 5.3 was merged as PR #5 (merge commit `373f3aa`, 2026-10-08). The branch `p5.3-auth-shell` still exists on `origin`, like the other two.
- 6.1 was merged as PR #6 (merge commit `ddff0a3`, 2026-10-08). The branch `p6.1-pact-wizard` still exists on `origin`.
- 6.2 was merged as PR #7 (merge commit `88d0da4`, 2026-10-08). The branch `p6.2-today-screen` still exists on `origin`.
- 6.3 was merged as PR #8 (`a54f2d7`), 6.4 as PR #9 (`d4e89b2`), 6.5 as PR #10 (`93d095a`), 6.6 as PR #11 (`e8121ad`), 6.7 as PR #12 (`72b934b`), 7.1 as PR #13 (`fa114b0`), 7.2 as PR #14 (`c6bb76b`) and 7.3 as PR #15 (`7e495f7`). 8.1 was merged as PR #16 (`728c86c`) after several GitHub runs (see PLAN 8.1). 8.2 was merged as PR #17 (`f0c4e37`). 8.3 was merged as PR #18 (`dae5f85`). 8.4 lives on `p8.4-load-test` (`b47ec02`, plus a docs commit), cut from `main` after that merge; it is not pushed or merged yet.
- `p2.3` is far over the ~600-line PR guideline in `AGENTS.md`. It is split into three commits (contract fixes, service layer, handlers) so it can be reviewed commit by commit. `p3.1` is four commits (service relay and reminders, worker, CLI and air, docs). `p3.2` is service claims and the invite event, the `notify` package, the worker email tasks, a copy fix, and docs.
- Commit messages carry no Claude attribution lines (the project owner's rule).

## 3. How to verify the current state

Prerequisite on this Windows machine: Go auto-switching is broken (installed Go 1.25.3, `go.mod` needs 1.26.0), and ffmpeg/libvips (winget) are only on the PATH of terminals opened after they were installed. Before any Go command, `bun run lint`, `bun run test`, or `bun run codegen`, run in the shell:

```bash
source scripts/dev-env.sh   # prepends the Go 1.26 toolchain and the ffmpeg/libvips bin dirs, prints what it found
```

Infra must be up (`bun run infra:up`): Postgres, Redis, Garage, Mailpit. Then:

```bash
bun run lint                                                  # scripts tsc, OpenAPI lint, gofmt, go vet
bun run test                                                  # bun script tests + Go unit tests (no DB needed)
bun run codegen -- --check                                    # generated code is fresh (run after committing)
cd apps/server && go test -race -count=1 -tags=integration ./...   # everything, real Postgres + Redis
```

Last result (2026-10-08, end of 4.2): all of the above green. For the integration run use `go test -race -p 3 -count=1 -tags=integration ./...`: without `-p 3` the linker ran out of memory once on this machine. The media tests skip (with a message) when ffmpeg/libvips are not on PATH, so check the output does not say SKIP before trusting a green run.

Integration tests create a throwaway database per test (`tepati_test_<random>`, dropped afterwards) and use Redis logical DBs: **auth uses 15, http uses 14, jobs uses 13**. Packages run in parallel and each flushes its DB, so **a new package that uses Redis in tests must take its own index** (`testdb.RedisIn(t, n)`; next free is 12).

Run it all: `bun run db:migrate`, then `bun run dev:hybrid` (infra, then the API and the worker through air; no web app yet). In a second shell, `bun run db:seed` creates an active pact with overdue check-ins, and `bun run ctl -- pact show <id>` shows the worker marking them `missed` within about a minute. Worker metrics: `curl localhost:9091/metrics`. Stop `dev:hybrid` with Ctrl+C (when it runs in a background shell, kill the `air` and `tmp/*.exe` processes). `source scripts/dev-env.sh` must have been run in the shell that starts `dev:hybrid`, because air runs `go build` and the worker calls ffmpeg/vips.

## 4. What exists

### Backend (`apps/server`, Go module `github.com/andi-frame/lockedin/apps/server`)

| Package | Role |
|---|---|
| `internal/config` | env parsing and validation (`caarlos0/env`), lists every missing var at once |
| `internal/domain` | pure rules, no I/O: `Terms` (validate, canonical hash), deadlines per timezone, the check-in FSM `Transition(ci, ev, now, ctx)` returning effects, ledger clamp math, proof-doc allow-list, `Clock` (`SystemClock`, `FakeClock`), `ValidReason` |
| `internal/store` | sqlc-generated queries and `WithTx`. Queries live in `db/queries/*.sql`, migration in `db/migrations/` |
| `internal/service` | every state change, one transaction each. `pacts.go` (lifecycle), `checkins.go` (transitions, `SweepDeadlines`, `ClosePacts`), `payouts.go` (mark paid, confirm), `outbox.go` (`RelayOutbox`), `reminders.go` (`SendReminders`), `mailing.go` (`Claim*Email`/`Release*Email`), `uploads.go` (`CreateUpload`, `CompleteUpload`, `GetAttachment`, `ProcessAttachment`, `GiveUpAttachment`, `CollectUploads`), `read.go` (membership-filtered views, keyset pages, `my_actions`) |
| `internal/auth` | argon2id, Redis sessions (hashed token), CSRF double-submit, login limiter, `RequireUser`, `UserFromContext` |
| `internal/http` | the Fiber app. `server.go` (middleware stack), `problem.go` + `statuses.go` (errors), `idempotency.go`, `ratelimit.go`, `observe.go` (access log + Prometheus), `health.go`, `handlers*.go` + `mappers.go` + `cursor.go` (the strict-server implementation) |
| `internal/http/api` | **generated** by oapi-codegen from `api/openapi.yaml`. Never edit |
| `internal/jobs` | the asynq worker. `jobs.go` (task types, `Schedule()` table, `Settlement` interface, handlers), `mail.go` (email tasks and `Mail` dependencies), `media.go` (`media:process`, `uploads:gc`, `MediaQueue` for the API), `worker.go` (`Run`: server + scheduler + metrics listener, graceful stop), `metrics.go` (per-instance Prometheus registry and the asynq queue collector) |
| `internal/media` | files in, files out, no database: `sniff.go` (magic bytes to MIME and kind), `media.go` (`Processor.Process`: image to WebP 2048 plus 480 thumbnail via `vips thumbnail`, video to H.264 720p plus poster via ffmpeg/ffprobe, pdf as is; `RejectError` for unacceptable input). Tests generate their own fixtures |
| `internal/storage` | `BlobStore` (`storage.go`: buckets `Staging`/`Media`, `ValidKey`, errors), `s3.go` (Garage via aws-sdk-go-v2, path-style, separate public endpoint for signing), `fs.go` (local dir plus HMAC-signed URLs and `ServeHTTP`), `factory.go` (`FromConfig`). `contract_test.go` is one suite run against both drivers; the s3 run needs Garage (integration tag) |
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

**Phase 4.2**
- **Flow:** `createUpload` (checks membership, pact `active`/`settling`, declared kind and type, size limit, per-pact quota, queue depth; inserts `awaiting_upload`; returns the presigned PUT) -> browser PUTs to storage -> `completeUpload` (owner only; `Head` must match the declared size or the object is deleted and the attachment rejected; sets `uploaded`; enqueues `media:process` after the commit) -> worker -> `ready` or `rejected` -> `getAttachment` (any pact member; signed GET URLs valid 5 minutes, `thumb` for images, `poster` for videos).
- **The real file type is sniffed from magic bytes** (`media.Sniff`), and the declared kind must match it. A PNG sent as video, SVG or HTML sent as an image, or an executable are all rejected with a reason in Indonesian. HEIC and AVIF are recognised and handed to libvips; whether they decode depends on the libvips build (the winget build was not tested with a HEIC file).
- **`RejectError` vs other errors:** a file the tools cannot read is a rejection; a cancelled context or a missing binary is a plain error and the job retries. `TestProcessAttachmentRetriesWhenInterrupted` guards this.
- **Idempotency:** `completeUpload` on an attachment past `awaiting_upload` just returns it; `media:process` claims `uploaded|processing` so a crashed attempt is retried; the task id is the attachment id.
- **Media keys:** staging `<pact>/<attachment>`, media `<pact>/<attachment>.webp|.mp4|.pdf`, thumbnails `<pact>/<attachment>_thumb.webp` (video poster is `_thumb.jpg`).
- **Limits:** raw size per kind and 40 MP are checked before and by the tools; duration is checked with ffprobe and again on the output, and ffmpeg is capped with `-t limit+1`; a transcoded video over 50 MB is rejected. The pact quota is a soft check (not atomic under concurrent uploads).
- **Proofs and attachments:** `submitProof` already requires attachments to be `ready` and owned by the submitter in that pact (3.x); nothing about that changed.

**Phase 4.1**
- **Garage enforces the signed `Content-Length`, `Content-Type` and expiry** (403 otherwise). ADR-0005 has the evidence, including a mutation check. Presigned mode is the default; `UPLOAD_MODE=proxy` exists in config but nothing implements it. Build it only if that test ever fails in a deployment.
- **One `BlobStore` interface, buckets as constants** (`Staging`, `Media`); the s3 driver maps them to `S3_BUCKET_STAGING`/`S3_BUCKET_MEDIA`. Keys must pass `storage.ValidKey` (letters, digits, `.`, `_`, `-`, single `/`). Generate keys from uuids; never from user input.
- **`PresignedPut.Headers` must be sent by the client as given** (at least `Content-Type`); the browser adds `Content-Length` itself. The upload intent response returns them.
- **The fs driver is for native dev.** Its URLs point at `http://localhost:$API_PORT/api/v1/blob/...` and are signed with `SESSION_SECRET`. The API mounts `FS.ServeHTTP` before the session gate only when the driver is fs (`Deps.Blob`).
- **`cmd/api` and `cmd/worker` both call `storage.FromConfig`** and pass the store to the service with `WithUploads` (4.2).

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
- **Uploads were stubbed in Phase 2** (`503 server.unavailable`) and are implemented in 4.2. They still answer 503 when the service has no storage configured (`ErrUploadsOff`).
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

- `problem.errors[]` (per-field detail) is in the contract but never populated; validation failures carry a text `detail` only. Phase 6.1 (the wizard) will want structured errors from `Terms.Validate`.
- `Service.Today` loads all of a user's pacts and filters `active|settling` in Go. Fine now; add a status filter to the query once completed pacts pile up.
- Server-level rejects (for example an oversize body) are handled by Fiber below the middleware, so they are not access-logged.
- Idempotency fails open if Redis is down (logged); the DB's own idempotency keys and status guards still protect money.
- Email: no `sent`/bounce tracking beyond `emailed_at`, no unsubscribe or per-user preferences (not in SPEC), and the `en` locale is not rendered (every email is Indonesian; `users.locale` is read but unused). Email copy lives in Go (`internal/notify/render.go`), not `messages/id.json`.
- Production SMTP (TLS, auth) is untested against a real provider; only Mailpit and a fake server were used. `smtps://` and `user:pass@` are implemented.
- The invite email is the only place the plaintext token is sent. If the email is lost (enqueue failure, crash after claim), the backer still has the link from the `Propose` response; there is no "resend invite" endpoint.
- Native dev needs `ffmpeg`, `ffprobe`, `vips` and `vipsheader` on PATH. They were installed with winget on the dev machine on 2026-10-08, so only *new* terminals have them. Shells that were already open (including the one running `dev:hybrid`) do not.
- A `go test -race ./...` of the whole module can run out of memory on the dev machine (`fatal error: runtime: cannot allocate memory` during a link). It passed with `-p 3`.
- No ClamAV or content scanning: a PDF is only checked for the `%PDF-` header and stored as is (SPEC §8). Videos keep working only for H.264/HEVC/VP9 inputs ffmpeg can decode.
- Uploads have no per-user rate limit beyond the global 30/min on `POST /uploads`, and no resumable upload; a dropped connection means starting that file again.
- The worker has `/metrics` and `/healthz` only, no `/readyz`. Fine for now; add one with the compose healthchecks (7.2).
- `tepati_settlement_transitions_total` is by job type, see §5.
- No test-clock endpoint (`CLOCK_OVERRIDE` is config only), by the owner's decision. The e2e gets time-dependent states from seed scenarios built through the service (`today`, `passbook`), and moves time with `tepatictl advance --pact <id>`, which ticks one pact's open check-ins with a clock set just past their deadline (6.4).
- `getToday` returns `review_queue_count` without rows, and `TodayCheckIn` has no commitment or rest-day count, so the Today page also calls `getReviewQueue` and `getPact` per active pact. Folding that into `getToday` would be a contract change.
- context7 MCP was never authenticated in these sessions; asynq, Prometheus and aws-sdk-go-v2 APIs were checked by reading module source under `~/go/pkg/mod`. For Phase 5 (Next.js 16, Tailwind v4, next-intl, Tiptap 3) read the official docs, or authenticate context7, before relying on memory of those APIs.

## 7. Brief for Phase 5 (web foundation) and the start of Phase 6

**Read first:** `docs/PLAN.md` Phase 5 (tasks 5.1 to 5.3) and its opening paragraph, `.impeccable/surfaces/apps-web-src-app-app.md` (the direction contract), `docs/design/README.md`, `docs/ARCHITECTURE.md §7`, and `AGENTS.md` "UI work". Before any UI code load the *impeccable* skill, then read `~/.claude/skills/impeccable/reference/craft-floor.md`, and use *taste-skill* sections 3, 4.4-4.6, 6 and 9 as a checklist. Use context7 for Next.js 16, Tailwind v4, next-intl and TanStack Query (it needs authenticating first, see §6).

**Backend is complete for the MVP loop** (phases 0-4): register/login, pacts and the agreement flow, check-ins and review, settlement and payouts, notifications and email, uploads and media. The OpenAPI contract is `api/openapi.yaml` and `apps/web/src/lib/api/schema.d.ts` is generated from it (`bun run codegen`).

**What 5.1 added (`apps/web`; see "What 5.2 added" below for the UI layer):**
- Next 16 App Router, `src/`, Turbopack, `output: "standalone"`. `bun run dev|build|start|typecheck|lint|test` inside `apps/web`; the root `bun run lint` and `bun run test` run them too, and `bun run dev:hybrid` starts it on `WEB_PORT`.
- `src/app/layout.tsx` (Geist and Geist Mono through the `geist` package, `NextIntlClientProvider`, `Providers`), `src/app/providers.tsx` (TanStack Query; 4xx are never retried), `src/app/page.tsx` (placeholder), `src/app/globals.css` (`@import "tailwindcss"` and the font variables only; tokens are 5.2).
- i18n: `src/i18n/request.ts` reads the `tepati_locale` cookie (`id` default, `en`), no URL prefix. Copy lives in `messages/id.json` and `en.json`; the `Errors` namespace is keyed by API error `code` and `errorMessageKey(code)` falls back to `Errors.unknown`. Add a message for every code a screen can show.
- API access: `src/lib/api/client.ts` (`createApiClient`), `browser.ts` (`api`, base `/api/v1`, same origin), `server.ts` (`serverApi()` for Server Components and actions: `API_INTERNAL_URL`, forwards the cookie jar and the CSRF cookie), `unwrap.ts` (turns an openapi-fetch result into data or a thrown `ApiError`), `errors.ts` (`ApiError` with `code`, `status`, `requestId`, `fieldErrors`, `retryAfterSeconds`), `csrf.ts`, `idempotency.ts`. Unsafe requests get `X-CSRF-Token` and an `Idempotency-Key`; a caller that retries one action passes its own key.
- Dev: `next.config.ts` rewrites `/api/*` to `API_INTERNAL_URL` (default `http://localhost:8080`) outside production, so the cookies stay same-origin. In production Caddy does that.
- `apps/web/AGENTS.md` is written by `next dev` (Next's own agent rules: read `node_modules/next/dist/docs/` before relying on memory of Next 16). Next 16 differences already hit or to expect: `middleware.ts` is now `proxy.ts` (5.3's auth guard), `next lint` is gone (plain `eslint .`), `next build` uses Turbopack.

**What 5.2 added (the UI layer):**
- **Tokens:** `src/styles/tokens.css` is the token authority until DESIGN.md exists (PLAN 8.2). One `light-dark()` pair per colour, OKLCH, plus radius 6/10, shadows and keyframes in `@theme`. Only these tokens exist (no default palette, radii or shadows). Use `bg-ground`, `bg-surface`, `bg-sunken`, `text-ink`, `text-muted`, `border-rule`, `bg-cover`, `bg-primary`, `text-debit`, `text-credit`, `bg-stamp`, `bg-today`, `bg-member-a`, `bg-member-b`, `rounded-control`, `rounded-panel`, `shadow-panel`, `shadow-overlay`, and so on. `src/styles/tokens.test.ts` fails if a text pair drops below WCAG AA in either theme; add a pair there for every new text-on-background combination.
- **Theme:** system by default, or the `tepati_theme` cookie (`system|light|dark`) read by the root layout into `data-theme` on `<html>`; `ThemeToggle` writes it. `data-theme` on any wrapper forces a theme for that subtree.
- **Primitives** (`src/components/ui`): `Button` (variants `primary`, `secondary`, `ghost`, `decision`, `decision-quiet`; the last two are stamp violet and only for human decisions), `Input`, `Textarea`, `Field` (label above, hint, error below, wires `aria-describedby`), `Select`, `Dialog`, `Sheet` (`side="bottom"` on phones, `"right"` on wide screens), `Tabs`, `Tooltip` (provider is in `Providers`), `Toast` (`toast({title, description, tone})` from anywhere; `Toaster` is in `Providers`), `Badge`. `cn()` is `tailwind-merge` taught the token names.
- **Domain components** (`src/components`): `Amount` (`coins`, `direction` debit|credit|balance, optional `rate` for the Rupiah line, sizes sm to xl), `Countdown` (`until`, optional `serverNow`, `onCover`; digits in fixed cells, one polite live region per minute), `StatusChip` (a `Record` over the generated `CheckInStatus`, so a new API status fails the typecheck until it has a chip), `MemberLine` (monogram and name in the member's fixed colour; `memberSlot(id, ids)` in `src/lib/member.ts` picks the slot from the sorted ids). Pure logic is in `src/lib/format.ts` (Intl `id-ID`, BigInt Rupiah), `countdown.ts` and `member.ts`, all table-tested.
- **Kitchen sink:** `/dev/kitchen-sink` (404 in a production build) shows everything with a fixed server clock and synthetic data. Use it to look at a primitive in both themes (`ThemeToggle` in its header, or emulate `prefers-color-scheme`).
- **Gotchas found:** next-intl message keys cannot contain `.` (use `auth_email_taken` for the code `auth.email_taken`; `errorMessageKey` does the mapping). Big heredocs fail in Git Bash: write component files with the Write tool. The Playwright MCP only writes screenshots under the repo (use `.playwright-mcp/`, gitignored), and an element screenshot with `section >> nth=N` is the way to look closely at one part of a long page. The Next dev indicator badge overlaps content in screenshots; it does not exist in production.
- **Not built yet:** a wordmark (text wordmark until one exists).

**What 5.3 added (sign-in and the shell):**
- **Guard:** `src/proxy.ts` (Next 16's `middleware`) redirects a request without a `tepati_session` cookie to `/login?next=<path>`, and `/` to `/login` or `/today`. Public: `/login`, `/register`, `/dev/*`. The pure rules and `safeNext` (same-site paths only, no open redirect) are in `src/lib/auth/guard.ts` with table tests. The proxy only sees the cookie, so the `(app)` layout calls `/me` through `getCurrentUser()` (`src/lib/auth/session.ts`, server only) and redirects on 401; `/login` and `/register` do the reverse for a live session. The proxy must not redirect signed-in visitors away from `/login` (a stale cookie would loop). Look for `proxy.ts: Nms` in the dev log to know it ran.
- **Pages:** `(auth)` group (`/login`, `/register`: cover band with the text wordmark, form on the ground) and `(app)` group (`/today`, `/pacts`, `/review`, `/settings`). The shell is `src/app/(app)/layout.tsx`: a cover-teal rail with guilloche from `lg` up (nav, unread bell, name and email, sign out) and a fixed bottom tab bar below `lg`; both read `navItems` and `isActive` from `src/lib/nav.ts`. Pages inside the shell only provide their own `<h1>` and content; `max-w-5xl` is applied by the layout. `error.tsx` is the error boundary for the group.
- **Forms:** `LoginForm` and `RegisterForm` (`src/components/auth`) validate with `src/lib/auth/forms.ts` (`validateLogin`, `validateRegister`, `formErrorFromApi`, which maps API codes to fields; wrong credentials stay form-level on purpose), call `api` through `unwrap()`, focus the first invalid field, and clear the TanStack cache on sign-in and sign-out. Copy keys are full paths (`Auth.emailInvalid`, `Errors.auth_email_taken`).
- **Placeholders:** Hari ini, Kontrak and Tinjau show an honest empty state until Phase 6; Pengaturan is real. The bell shows the unread count and is not a button yet.
- **e2e:** `@playwright/test` in `apps/web`, `playwright.config.ts` (desktop 1440 and mobile 390 projects, one worker), `tests/e2e/auth.spec.ts`. `bun run test:e2e` expects the stack to be running. The API allows 10 auth requests a minute per IP (not configurable); one full run uses about 7, so wait a minute between runs and keep new specs frugal (register one account per spec, reuse `storageState` where you can).

**What 6.1 added (pacts: create, invite, sign):**
- **Pure logic** in `src/lib/pact` (all table-tested): `dates.ts` (plain `YYYY-MM-DD` strings; "today" is always asked for a named zone and an instant), `draft.ts` (the wizard's working copy with numbers as strings, `validateStep` per step mirroring SPEC §4 and `Terms.Validate`, `buildTerms`, `draftFromPact`; the doer slot is keyed by the nil UUID until someone joins), `summary.ts` (the plain-language terms as `{key, params}` lines; wording is in `messages/*.json` under `Terms`, and a test checks every key and every `{placeholder}` exists in both languages), `agreement.ts` (`checkSignature`, `inviteUrl`, `agreementState`).
- **Screens:** `/pacts` (list, minimal), `/pacts/new` and `/pacts/[id]/edit` (the same `Wizard`: Dasar, Komitmen, Koin dan aturan, Tinjau ketentuan), `/pacts/[id]` (signatures, terms, sign form, invite tools, edit link; nothing about the ledger yet) and `/invite/[token]` (private on purpose; the proxy sends a signed-out invitee to `/login?next=...` and `safeNext` brings them back).
- **Signing** is stamp violet (`SignForm`: tick, then type your display name exactly, case-insensitive). The invitee's button does `joinInvite` then `acceptPact` with the new hash. Any edit calls `updatePact`, which clears both signatures; the pact page says so when `terms_version > 1` and nobody has signed.
- **Retries:** the wizard keeps one `Idempotency-Key` per logical action and per payload; if Propose fails after the draft was created, a retry does not create a second pact (it updates the draft if the person changed something, then proposes again).
- **e2e:** `pact-create.spec.ts` (desktop only; two contexts; registers two users). With the other specs a full `bun run test:e2e` costs 9 of the 10 auth requests a minute.
- **Local only:** if port 8080 is busy, set `API_PORT` and `API_INTERNAL_URL` in `.env` (gitignored) and run the e2e with `E2E_API_URL`.

**What 6.2 added (the Today screen):**
- **Page:** `/today` (`src/app/(app)/today/page.tsx`, with a `loading.tsx` skeleton) calls `getToday`, `getReviewQueue` (5 rows), the unread count, and `getPact` for each active pact. Pure rules are in `src/lib/today.ts` (`ledgerEntry`, `restLeft`, `groupToday`, `nextDeadline`, `dayLabel`) and `clockIn` in `src/lib/pact/dates.ts`, all table-tested.
- **Components** (`src/components/today`): `DeadlineBand` (cover teal with guilloche, the date in highlighter yellow, the countdown to the soonest open check-in, the bell on a phone), `PactSection` (one section per pact, nearest deadline first: commitment, status chip, countdown, cutoff and grace, rest day; sending proof is a disabled button until 6.3), `RestButton` (confirm dialog, `declareRest`), `ReviewRows` (links to `/review`), `MiniPassbook` (last three ledger lines, signed amounts with the D/K mark, saldo with the Rupiah line). Desktop shows sections and review rows on the left and passbooks on the right; a phone stacks them in that order.
- **Test data:** `tepatictl seed --scenario today` (`bun run db:seed -- today`) creates new users and four active pacts with today's check-in open, submitted, approved and missed (the backer's own proof waits for the doer to review). `bun run test:e2e` seeds two doers, one per project. See `docs/PLAN.md` 6.2 for why this and not a clock endpoint.
- **Config:** `AUTH_RATE_LIMIT_PER_MIN` (default 10, at most 10 in production). Put `AUTH_RATE_LIMIT_PER_MIN=200` in the local `.env` to run the whole e2e suite at once; without it a full run needs a minute's wait and the two Today specs can push it past 10.

**What 6.3 added (writing and sending proof):**
- **Route:** `/pacts/[id]/days/[date]` (`src/app/(app)/pacts/[id]/days/[date]/page.tsx`) finds the signed-in person's check-in for that date (`listPactCheckIns` with `from` and `to`), loads `getCheckIn`, and shows the editor while `my_actions` has `submit` or `edit_proof`, otherwise the status and a way back. 6.5 builds the read-only detail on the same route. Today links to it ("Kirim bukti", and "Ubah bukti" while a submitted proof can still be changed).
- **Pure rules** in `src/lib/proof` (all table-tested): `doc.ts` (`analyzeDoc`: a mirror of the server's `ParseProofDoc`, so the word count that gates the button is the server's own; the allow-list; `normalizeLink`), `extensions.ts` (`proofExtensions()`: StarterKit with underline and trailing node off, headings 2 and 3, http(s) links, task lists, CharacterCount; a test checks the schema against the allow-list), `rules.ts` (`checkEvidence`, mirror of `Evidence.Check`), `files.ts` (`checkFile`, `declaredMime`, `roomLeft`: SPEC §8 limits), `net.ts` (`putHeaders`, `pollDelay`, `targetSize`), `tray.ts` (the attachment state machine: preparing, uploading, processing, ready, rejected, failed).
- **Browser code:** `compress.ts` (canvas, WebP), `upload.ts` (`uploadAttachment`: `createUpload`, XHR PUT with progress and exactly the returned headers minus Content-Length, `completeUpload`, poll `getAttachment`), `components/proof/use-attachments.ts` (one abortable job per file, retry, remove).
- **Components** (`src/components/proof`): `ProofEditor` (toolbar that wraps onto rows on a phone, link form, files pasted or dropped on it go to the tray), `AttachmentTray` (pick, camera, drop, per-file stage, rejection reason, retry), `ProofForm` (the commitment, the countdown, word and file counters against the rules, why the button is off, one idempotency key per payload).
- **Seed:** `tepatictl seed --scenario today` now has five pacts; "Today: rules" needs 10 words and one ready attachment. `bun run test:e2e` seeds one doer per spec and per project (`E2E_TODAY_EMAIL_*`, `E2E_PROOF_EMAIL_*`, `E2E_TODAY_PASSWORD`). The fixtures `tests/e2e/fixtures/photo.png` and `clip.mp4` are tiny files made with ffmpeg.
- **Needs running:** the media worker, ffmpeg and libvips (STATUS §6), because "Siap" only appears when the server has processed the file.

**What 6.4 added (the pact page):**
- **Route:** `/pacts/[id]` keeps the agreement view for `draft`, `proposed` and `scheduled`, and shows `RunningPact` (`src/app/(app)/pacts/[id]/running.tsx`) for `active`, `settling` and `completed`: the cover-teal header band with the guilloche (title, status, dates), then two columns (one on a phone): the coin book, and beside it the calendar, the members and the terms in a `<details>`.
- **Pure rules** (table-tested): `src/lib/passbook.ts` (`PAGE_SIZE` 20, `mergeLines`, `arrivals`, `rollValue`), `src/lib/pact/calendar.ts` (`monthOf`, `shiftMonth`, `monthsBetween`, `startMonth`, `weeksOf` Monday first, `dayCells`).
- **Client code** (`src/components/pact`): `usePactLive` (first page from the server, older pages by cursor, a refresh of the first page and the calendar every 30 s and when the tab comes back; the lines newer than any shown are the arrivals), `Passbook` (date, keterangan with the member's name under it in their line colour, debit, kredit, saldo; an observer loads the next page, a button does the same), `RollingAmount` (the big saldo rolls to the new figure), `PactCalendar` (month grid with prev and next, the status chip's symbol plus a tone, a bar in the member's colour, today in the highlighter, a legend), `PactLive` (composes them).
- **Print motion:** `.print-line` (slide, 240 ms) and `.print-digits` (the digits type left to right in 320 ms, in as many steps as characters, `--chars`) in `globals.css`; the saldo rolls after 320 ms; under reduced motion the global rule makes durations and delays zero. A polite live region announces the new line. The balance and every `balance_after` are the server's; nothing is summed in the browser.
- **Seed and test clock:** `tepatictl seed --scenario passbook` (new users `passbook-<tag>-*@tepati.test`, one active pact "Passbook: history" with 24 days of lines, 33 in all, the backer's proof for today already sent and the doer's still open) and `tepatictl advance --pact <id>` (see §6 gaps). `Service.SweepCheckIns(ids)` is the seam that lets `advance` tick one pact only. `bun run test:e2e` seeds one passbook pact per project (`E2E_BOOK_EMAIL_*`).

**What 6.5 added (reviewing and the check-in detail):**
- **Routes:** `/review` (server page, `getReviewQueue`, 20 per page by `?cursor=`: member, pact, date, word and file counts, a countdown to the review deadline, each row linking to the check-in). `/pacts/[id]/days/[date]` now has two faces: the 6.3 editor (the viewer's own check-in when `my_actions` has `submit` or `edit_proof`, unless a decision is also possible, then the detail comes first and `?edit=1` opens the editor) and `CheckInDetailView` for everything else. `?of=<user id>` picks whose check-in of that day (default: the viewer's).
- **Pure rules** in `src/lib/checkin` (table-tested): `reason.ts` (`reasonState`: at least 10 characters after trimming, counted as characters, at most 1000), `timeline.ts` (`timelineEntries`: who acted from the viewer's side and the tone `power`, `human`, `system` or `plain`; `overrideDecision`; `nextDeadline`: the one deadline that matters per status).
- **Components** (`src/components/checkin`): `ProofView` (Tiptap static renderer with `proofExtensions()`), `AttachmentGallery` (lightbox, video player, PDF links), `DecisionTimeline` (backer power in stamp violet with the label "Kuasa penyokong"), `CheckInActions` (exactly `my_actions`: approve is one click, reject, override, dispute and resolve open a dialog with a reason; one idempotency key per payload; the server's refusal shows in the dialog), `CheckInDetailView` (header with status, an override note for both members, the deadline with a countdown, the actions, the proof with its word count and version history, attachments, the history, a link to the other member's check-in).
- **Styling:** `.proof-prose` in `globals.css` now gives the proof body its typography in the editor and in the read-only view (it had none before).
- **Seed:** `tepatictl seed --scenario review` (new users `review-<tag>-*@tepati.test`): a backer who does not commit, three pacts "Review: approve", "Review: dispute" (submitted) and "Review: override" (auto-approved, 3 overrides left). `bun run test:e2e` seeds one pair per project (`E2E_REVIEW_BACKER_*`, `E2E_REVIEW_DOER_*`) and retries a seed up to three times (the passbook seed can race the worker).

**What 6.6 added (settlement and payout):**
- **Panel:** `SettlementPanel` (`src/components/pact/settlement-panel.tsx`) is drawn by `RunningPact` above the coin book whenever the pact has a payout (`settling` or `completed`): the amount the worker fixed (with the Rupiah equivalent), who marked it paid with the note and when, and who confirmed and when. For a pot that ran out it says nothing is owed.
- **Actions** (`payout-actions.tsx`, client): `MarkPaidButton` (backer, once, dialog with an optional note of at most 500 characters) and `ConfirmReceiptButton` (doer, dialog that says it cannot be undone and warns when the backer has not marked it paid; the doer's confirmation alone completes the pact). One idempotency key per payload; the server's refusal shows in the dialog; success refreshes the page.
- **Pure rules:** `src/lib/pact/settlement.ts` (`settlementState`: phase, amount, who can do what; `noteState`), table-tested.
- **Seed:** `tepatictl seed --scenario settlement` (new users `settlement-<tag>-*@tepati.test`): two pacts, "Settlement: paid first" and "Settlement: doer only", five days that ended five days ago, 900 coins owed, settled by the real `ClosePacts`. `bun run test:e2e` seeds one pair per project (`E2E_SETTLE_*`).

**What 6.7 added (notifications inbox and settings):**
- **Inbox:** `/notifications` (server page, `?cursor=` for older ones) lists my notifications newest first, 20 a page. Each row has an icon, one sentence for its kind (`Notifications.kinds`, 22 kinds, both languages), the pact title, the time in the visitor's zone, and links to what it is about. The payload carries ids only, so the page joins titles from `GET /pacts` and, for kinds that open a check-in, the day and owner from `GET /check-ins/{id}` (`notificationHref` falls back to the pact when that lookup fails).
- **Read state:** `NotificationList` (client) posts the ids on the page to `/notifications/read` once after hydration and refreshes the route, so the bell loses its count; the "Baru" labels stay for that visit. `NotificationBell` is now a link to the inbox (it was an inert readout).
- **Pure rules:** `src/lib/notification/inbox.ts` (`KIND_TARGET`, `payloadRefs`, `notificationHref`, `checkInIdsToResolve`, `unreadIds`), table-tested, plus a test that every kind has copy in both languages.
- **Settings:** unchanged controls. The page now says the account cannot be changed from the app yet, and has a Notifikasi section that says what is sent where and links to the inbox.
- **Open question for the owner:** the API has no `PATCH /me` and no notification preferences. Should the MVP get them (name, time zone, language; per-kind email on or off), or stay without? Not decided; nothing was invented.

**What 7.1 added (Dockerfiles):**
- `deploy/docker/server.Dockerfile`: Go 1.26 build stage (module cache and build cache mounts), then `debian:bookworm-slim` with `ffmpeg libvips-tools ca-certificates tzdata`, user 10001. One image holds `api`, `worker`, `tepatictl` and `goose` (in `/usr/local/bin`) and the migrations in `/app/migrations`; the default command is `api`, compose picks the others. The version is stamped with `-X ...buildinfo.Version`. 989 MB.
- `deploy/docker/web.Dockerfile`: `oven/bun:1.4` builds with Next `output: "standalone"`, `oven/bun:1.4-slim` runs `bun apps/web/server.js` as user 10001 on port 3000. `API_INTERNAL_URL` is read at run time. 339 MB. In production there is no `/api` rewrite, so the API has to be reached through Caddy (7.2).
- `bun run deploy:build -- --env staging|production [--only server|web]` (`scripts/deploy-build.ts`, rules in `scripts/lib/images.ts`, tested) builds both and tags `tepati-<image>:<env>` and `tepati-<image>:<git describe>`. Each Dockerfile has its own `.dockerignore` next to it.
- Checked by running the images against the dev infra: api `/healthz` and `/readyz` green, worker starts and stops cleanly, goose lists the applied migrations, the web image serves `/login` and static chunks.

**What 7.2 added (compose, Caddy, deploy scripts):**
- **Compose:** `deploy/compose.yaml` has the `app` (migrate, api, worker, web), `edge` (caddy), `tools` (asynqmon 0.7.2, loopback only) and `backup` profiles besides `infra`. `compose.dev.yaml` swaps the app services for a Go dev image (`deploy/docker/dev-server.Dockerfile`: air, goose, ffmpeg, vips) and `oven/bun` with source bind mounts and the shadowing volumes of RUNNING §4. `compose.prod.yaml` adds `read_only`, tmpfs, resource limits, drops every host port but Caddy's and puts Mailpit behind a `mail-sandbox` profile.
- **Edge:** `deploy/caddy/Caddyfile`: `<DOMAIN>` (web, `/api/*` with a 2 MB cap, `/healthz`, `/readyz`, security headers and a CSP) and `media.<DOMAIN>` (Garage S3, 210 MB cap). api and web are found through DNS so `deploy:scale` works.
- **Scripts:** `deploy:up` and `deploy:scale` (`scripts/deploy-up.ts`, `deploy-scale.ts`, pure rules in `scripts/lib/deploy.ts`, tested); `garage-init.ts` takes a target; `ctl.ts` runs inside a deploy's api container with `TEPATI_CTL_PROJECT`; `e2e.ts` and the Playwright config accept `https://localhost`.
- **Server:** `tepatictl probe <url>` (health checks) and `tepatictl storage-init` (CORS from inside the network); `RATE_LIMIT_PER_MIN`; the login attempt limit follows `AUTH_RATE_LIMIT_PER_MIN`. **Web:** `serverApi()` forwards the visitor's `x-forwarded-for`, so rate limits are per visitor.
- **Backups:** `deploy/backup` (nightly `pg_dump` into the Garage bucket `tepati-backups`, 14 daily and 8 weekly). Restore steps are in RUNNING §6; there is no RUNBOOK yet.
- **Not done:** a real domain, SMTP and TLS (nothing was deployed outside this machine); a restore into a scratch database was not rehearsed (the dump was written and the pruning verified, `pg_restore` was not run); the e2e `proof` spec ran through Caddy but the CSP was only checked by that run, not by a manual look at the browser console.

**What 7.3 added (native mode):**
- **Pre-flight** (`scripts/lib/checks.ts`, tested): a real Postgres login (wrong password, missing database and refused connection each get their own sentence; the version is shown) and a Redis version check (7.0 or newer, because the login limiter uses `EXPIRE ... NX`), besides ffmpeg, ffprobe, vips and vipsheader. All problems are listed at once.
- **Migrations:** `dev:native` applies them to the native database before starting the apps.
- **`.env.native`:** `deploy/env/.env.native.example` is the template; `TEPATI_NATIVE=1` makes `db:migrate`, `ctl` and the e2e seeds use it (`layerNativeEnv`, `readRootEnv`, `pickDatabaseUrl` in `scripts/lib/env.ts`). Bun puts the Docker `DATABASE_URL` from `.env` into the environment, so in native mode the layered file wins over it.
- **e2e on a slow stack:** `E2E_TIMEOUT_MS` and `E2E_EXPECT_TIMEOUT_MS` (Playwright config and the per-spec timeouts).
- **Docs:** `docs/RUNNING.md` section 7 (native mode, with a throwaway Postgres recipe).
- **Verified** with a throwaway PostgreSQL 18 cluster, Redis 8 built in WSL, and the winget ffmpeg and vips: the full e2e passed natively with the fs driver. **Not verified:** Memurai, Postgres installed as a service, Postgres 16.
- **Local leftovers (gitignored):** `.env.native` (ports 3010, 18090, 9092, Postgres 5440, Redis 6390), `.data/pg18-native/` (the throwaway cluster, stopped), and Redis 8 built under `/opt/src/redis-8.10.2` in the WSL Ubuntu (stopped; the Redis 6.2 already there is untouched).

**What 8.1 added (CI):**
- `.github/workflows/ci.yml`: `check` (lint, `codegen --check`, unit tests with ffmpeg and libvips installed), `integration` (`setup`, `infra:up`, `test:integration`), `e2e` (`deploy:env-local`, `deploy:build`, `deploy:up`, then the Playwright suite against `https://localhost`, with artifacts and stack logs on failure). Runs on pull requests and pushes to `main`; one retry on CI.
- `bun run deploy:env-local` (`scripts/deploy-env-local.ts`, `scripts/lib/localenv.ts`): a ready `.env.staging` for a deploy on this machine or a runner.
- `docs/RUNNING.md` section 8 explains the jobs and how to reproduce `e2e` on a laptop.
- **Run on GitHub (PR #16):** all three jobs green on the second run (about 5 minutes each in parallel). The first run found a real media defect (a one-frame video got no poster; fixed in `internal/media`). One e2e test (`proof.spec.ts`, desktop) failed once in a full local run and passed 3 of 3 alone; it did not fail on GitHub and is not explained.

**What 8.2 added (design finish):**
- **Review:** an inline finish review (no subagent, by the owner's standing rule) over 44 captures (11 screens, 390 and 1440, light and dark) in `.impeccable/review/screens/` (gitignored). Two material fixes in two rounds: text links are underlined at rest (nine places), and the guilloche is off the rail and stays on the header bands. Details and what was accepted are in PLAN 8.2.
- **`DESIGN.md` and `.impeccable/design.json`:** the design system as built (tokens read from `src/styles/tokens.css`, rules, components). `bun run lint` does not check them; if the tokens change, regenerate or edit both. They are the reference for any new screen, together with `.impeccable/surfaces/apps-web-src-app-app.md`.
- **Rules worth knowing:** red and green mean coins only; violet only for human decisions; yellow only for today; the guilloche only on header bands; text links are underlined; no nested cards, kickers, gradient text, hard shadows or glyph icons.

**What 8.3 added (guidelines audit):**
- The proof form and the new-pact wizard warn before the tab is closed or reloaded with unsaved work (`src/lib/unsaved.ts`); the sign-in, register and new-pact name fields focus themselves only with a mouse (`src/lib/use-focus-on-desktop.ts`); non-auth fields have `autocomplete="off"`; progress labels end with an ellipsis; `theme-color` follows the page ground and the chosen theme; touch and overscroll defaults are set. The accepted exceptions and the reasons are in PLAN 8.3.
- `DESIGN.md` now also lists the small type steps (small, micro, prose headings) and the selection colour; `impeccable detect` compares the code against it and reports nothing.

**What 8.4 added (load test):**
- `tests/load/today.js` (k6) and `bun run load` (`scripts/load.ts`, `scripts/lib/load.ts`, tested): seeds users in a deploy, runs `grafana/k6` on its network with SPEC §10 thresholds. See RUNNING §9.
- **Result on the owner's laptop** (not a 2 vCPU verdict, see PLAN 8.4): at 200 reads and 20 proof edits a second for five minutes, read p95 12.9 ms and write p95 23.6 ms with no errors; one api copy (1 CPU) saturates between roughly 350 and 450 requests a second, and `api=2` carries 550 a second at p95 16 ms and 24 ms.
- `tests/load/.users.json` is generated and gitignored. The staging env file used for the run has `RATE_LIMIT_PER_MIN=100000` (local only).

**Useful for the web work:**
- `bun run dev:hybrid` starts infra, api and worker. `bun run db:seed` and `bun run db:seed -- invite` give data to look at (seed users `seed-backer@tepati.test` / `seed-doer@tepati.test`, password `tepati-seed-1234`). Mailpit is at http://localhost:8025.
- Auth is cookie based with a CSRF header (`X-CSRF-Token`, value from the `tepati_csrf` cookie) on unsafe methods, and mutating calls take an `Idempotency-Key`. Details in `docs/adr/0010-api-conventions.md`.
- Upload client flow: `POST /uploads` -> `PUT` the file to `put_url` with exactly the returned `headers` (the browser adds `Content-Length`) -> `POST /uploads/{id}/complete` -> poll `GET /attachments/{id}` (1 s backing off to 5 s) until `ready` or `rejected`. With the fs storage driver `put_url` points at the API (`/api/v1/blob/...`), with Garage at port 3900; CORS in dev already allows `Content-Type`.
- Email links point at `/pacts/<id>`, `/review` and `/invite/<token>`; these routes must exist.
- The server does not check `starts_on` against the date: a pact proposed for tomorrow and signed after tomorrow starts with check-ins that are already overdue. The wizard only prevents choosing a start before tomorrow. Needs a rule in SPEC (refuse to schedule once `starts_on` has passed?) and a server check; not decided.

**Process reminders (from `AGENTS.md`):** one PLAN task per branch (`p8.5-security-review`), update `docs/PLAN.md` with the commit hash, update this file when behaviour changes, screenshots at 390 and 1440 px in light and dark for UI work, and refresh the knowledge graph with `/graphify . --update` (last refreshed after Phase 2). Do not start 9.1 in the same session as 8.5 unless asked.
