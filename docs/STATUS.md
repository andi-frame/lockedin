# Tepati: project status and handoff

Last updated: 2026-10-08, after task 5.3 (auth pages and app shell; the backend is complete and Phase 5 is done). This is the first thing to read when you start a new session, together with `docs/HANDOVER.md` (how to work in this repo and on this machine, whatever agent you are). `docs/PLAN.md` says *what* is next; this file says *where we are*, what exists, what was decided on the way, and what to watch out for. Update it at the end of every phase.

## 1. Where we are

| Phase | State | Notes |
|---|---|---|
| 0 Repository foundation | done | Bun workspace, env scripts, infra compose, dev orchestrator |
| 1 Backend core (Go) | done | Schema, pure domain, store, pact and check-in services, settlement sweep, auth |
| 2 API contract and HTTP | done | OpenAPI contract, Fiber app, all 31 operations |
| 3 Worker | done | 3.1 asynq worker, schedule, outbox relay, reminders, `tepatictl seed`/`pact show`, air files. 3.2 notifications and email (Mailpit) |
| 4 Uploads and media | done | 4.1 BlobStore drivers (s3 and fs, Garage enforces the signed length). 4.2 upload endpoints, `media:process`, `uploads:gc` |
| 5 Web foundation | done | 5.1 Next.js app on Bun (PR #3). 5.2 design tokens and primitives (PR #4). 5.3 auth pages, app shell, auth guard and the Playwright e2e: done on `p5.3-auth-shell` (not pushed or merged yet) |
| **6 Web features** | **next** | 6.1 new pact wizard and invite/accept flow |
| 6–9 | not started | Web features, Docker, quality gates, staging |

The first unchecked task in `docs/PLAN.md` is **6.1**.

## 2. Branch and merge state

Phases 2 to 4 are **merged into `main`**: pull request #1 (`p4.2-uploads` -> `main`) was merged on 2026-10-08 with a merge commit (`054e61a`), not squashed, so every commit hash recorded in `docs/PLAN.md` is in `main`'s history. Before the merge the work lived on seven **stacked** branches, each cut from the one before:

```
e4f987f (end of Phase 1) ─ p2.1-openapi ─ p2.2-fiber-middleware ─ p2.3-handlers ─ p3.1-worker-jobs ─ p3.2-notifications-email ─ p4.1-blobstore ─ p4.2-uploads ─ merge 054e61a (main)
```

- The seven task branches (`p2.1-openapi` ... `p4.2-uploads`) were deleted from `origin` and locally on 2026-10-08, at the owner's request, after the merge. Their commits are in `main`.
- 5.1 was merged into `main` as PR #3 (merge commit `f715daf`, 2026-10-08). The branch `p5.1-nextjs-app` still exists on `origin` (the owner has not asked to delete it).
- 5.2 was merged as PR #4 (merge commit `e189035`, 2026-10-08). The branch `p5.2-tokens-primitives` still exists on `origin`, like `p5.1-nextjs-app` (the owner has not asked to delete them).
- 5.3 lives on `p5.3-auth-shell` (four code commits, `a9f3db3..884e87d`, plus a docs commit), cut from `main` after the 5.2 merge. It is not pushed or merged yet. Cut 6.1 from it if it is still unmerged, otherwise from `main`.
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
- The worker has `/metrics` and `/healthz` only, no `/readyz`. Fine for now; add one with the compose healthchecks (7.1).
- `tepati_settlement_transitions_total` is by job type, see §5.
- No test-clock endpoint yet (`CLOCK_OVERRIDE`); the e2e work (5.3/6.x) needs it. The API and worker use `domain.SystemClock`.
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
- **Not built yet:** the passbook print animation (6.4), a wordmark (text wordmark until one exists), Tiptap (6.3).

**What 5.3 added (sign-in and the shell):**
- **Guard:** `src/proxy.ts` (Next 16's `middleware`) redirects a request without a `tepati_session` cookie to `/login?next=<path>`, and `/` to `/login` or `/today`. Public: `/login`, `/register`, `/dev/*`. The pure rules and `safeNext` (same-site paths only, no open redirect) are in `src/lib/auth/guard.ts` with table tests. The proxy only sees the cookie, so the `(app)` layout calls `/me` through `getCurrentUser()` (`src/lib/auth/session.ts`, server only) and redirects on 401; `/login` and `/register` do the reverse for a live session. The proxy must not redirect signed-in visitors away from `/login` (a stale cookie would loop). Look for `proxy.ts: Nms` in the dev log to know it ran.
- **Pages:** `(auth)` group (`/login`, `/register`: cover band with the text wordmark, form on the ground) and `(app)` group (`/today`, `/pacts`, `/review`, `/settings`). The shell is `src/app/(app)/layout.tsx`: a cover-teal rail with guilloche from `lg` up (nav, unread bell, name and email, sign out) and a fixed bottom tab bar below `lg`; both read `navItems` and `isActive` from `src/lib/nav.ts`. Pages inside the shell only provide their own `<h1>` and content; `max-w-5xl` is applied by the layout. `error.tsx` is the error boundary for the group.
- **Forms:** `LoginForm` and `RegisterForm` (`src/components/auth`) validate with `src/lib/auth/forms.ts` (`validateLogin`, `validateRegister`, `formErrorFromApi`, which maps API codes to fields; wrong credentials stay form-level on purpose), call `api` through `unwrap()`, focus the first invalid field, and clear the TanStack cache on sign-in and sign-out. Copy keys are full paths (`Auth.emailInvalid`, `Errors.auth_email_taken`).
- **Placeholders:** Hari ini, Kontrak and Tinjau show an honest empty state until Phase 6; Pengaturan is real. The bell shows the unread count and is not a button yet.
- **e2e:** `@playwright/test` in `apps/web`, `playwright.config.ts` (desktop 1440 and mobile 390 projects, one worker), `tests/e2e/auth.spec.ts`. `bun run test:e2e` expects the stack to be running. The API allows 10 auth requests a minute per IP (not configurable); one full run uses about 7, so wait a minute between runs and keep new specs frugal (register one account per spec, reuse `storageState` where you can).

**Useful for the web work:**
- `bun run dev:hybrid` starts infra, api and worker. `bun run db:seed` and `bun run db:seed -- invite` give data to look at (seed users `seed-backer@tepati.test` / `seed-doer@tepati.test`, password `tepati-seed-1234`). Mailpit is at http://localhost:8025.
- Auth is cookie based with a CSRF header (`X-CSRF-Token`, value from the `tepati_csrf` cookie) on unsafe methods, and mutating calls take an `Idempotency-Key`. Details in `docs/adr/0010-api-conventions.md`.
- Upload client flow: `POST /uploads` -> `PUT` the file to `put_url` with exactly the returned `headers` (the browser adds `Content-Length`) -> `POST /uploads/{id}/complete` -> poll `GET /attachments/{id}` (1 s backing off to 5 s) until `ready` or `rejected`. With the fs storage driver `put_url` points at the API (`/api/v1/blob/...`), with Garage at port 3900; CORS in dev already allows `Content-Type`.
- Email links point at `/pacts/<id>`, `/review` and `/invite/<token>`; these routes must exist.
- A test-clock endpoint (`CLOCK_OVERRIDE`) does not exist yet; the Phase 6 e2e tests (`today.spec.ts` needs open, submitted, approved and missed) will need one or a different way to move time.

**Process reminders (from `AGENTS.md`):** one PLAN task per branch (`p6.1-pact-wizard`), update `docs/PLAN.md` with the commit hash, update this file when behaviour changes, screenshots at 390 and 1440 px in light and dark for UI work, and refresh the knowledge graph with `/graphify . --update` (last refreshed after Phase 2). Do not start 6.2 in the same session as 6.1 unless asked.
