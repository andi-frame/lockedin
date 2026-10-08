# Tepati: Implementation Plan

This plan is written so that **any capable agent (for example Claude Sonnet 5.5) or human developer can pick up the next unchecked task and finish it without extra context**. Read `AGENTS.md` first.

Resuming? Read `docs/STATUS.md` first: it records where the project stands, decisions made so far, known gaps, and the brief for the next phase.

How to use it:
- Work **one task at a time**, in order, unless a task is marked ⇄ (parallel-safe with its siblings).
- Each task lists **Read**, **Do**, **Done when** (acceptance criteria), and **Verify** (exact commands). A task is finished only when every Verify command passes and you have looked at the output.
- Tick the box (`[x]`) and add the short commit hash in the same PR that completes the task.
- If a task turns out to be wrong or too big, edit this plan in your PR and say why. Don't silently deviate.
- Skills are named in *italics*. Load them with the Skill tool when the task says so.

Legend: ⇄ parallel-safe · 🔒 touches money/time invariants (extra review: run *superpowers:requesting-code-review* or `/code-review high`).

---

## Phase 0: Repository foundation

- [x] **0.1 Workspace skeleton**
  - Read: `docs/ARCHITECTURE.md §2`, `docs/RUNNING.md §3–4`.
  - Do: Create the root `package.json` (Bun workspace: `apps/web`, `scripts`), `.gitattributes` (LF for `*.sh *.toml *.go *.ts *.tsx *.yaml *.yml Caddyfile Dockerfile*`), `.editorconfig`, `.gitignore` (env files, `.data/`, `node_modules`, `.next`, `tmp/`, `graphify-out/cache`), and an empty `apps/`, `api/`, and `deploy/` tree as in the layout.
  - Done when: `bun install` succeeds on a clean clone.
  - Verify: `bun install && git status --short` shows only intended files.

- [x] **0.2 Env examples and setup script**
  - Do: Write `deploy/env/.env.example` with every variable from `ARCHITECTURE.md §9`, each with a comment. Add `.env.dev.example`, `.env.staging.example`, and `.env.production.example`. Write `scripts/setup.ts`: copy examples to `.env` (repo root, used by hybrid/native) and `deploy/env/.env.dev` only when missing, and fill `SESSION_SECRET`, `GARAGE_RPC_SECRET` (32-byte hex), `GARAGE_ADMIN_TOKEN`, and `GARAGE_METRICS_TOKEN` with crypto-random values.
  - Done when: running setup twice leaves files unchanged the second time.
  - Verify: `bun run setup && bun run setup` (the second run prints "exists, skipped").

- [x] **0.3 Infra compose (profile `infra`)**
  - Read: `docs/RUNNING.md §4–5` (mount rules are mandatory).
  - Do: In `deploy/compose.yaml`, add services `postgres:18`, `redis:8` (`--appendonly yes`), `dxflrs/garage:v2.4.1`, and `axllent/mailpit` (Garage bootstrap is a script, because the image has no shell), all with health checks and named volumes only. Add `deploy/garage/garage.toml`. Write `scripts/garage-init.ts`, which calls the `garage` CLI through `docker compose exec` and is idempotent (layout, key, buckets, bucket CORS for PUT from `APP_BASE_URL`), then writes `S3_ACCESS_KEY`/`S3_SECRET_KEY` into `.env` and `deploy/env/.env.dev`.
  - Done when: `bun run infra:up` from a fresh clone reaches healthy, and running it a second time changes nothing.
  - Verify: `bun run infra:up && docker compose -f deploy/compose.yaml ps` (all healthy). Then `bun run garage:init` a second time and confirm it prints no-op lines. Then `bun run s3:smoke` lists both buckets and round-trips an object.

- [x] **0.4 Dev orchestrator**
  - Do: Write `scripts/dev.ts`, which spawns named processes with coloured prefixes, forwards Ctrl+C, and exits non-zero if any child crashes. Modes: `docker`, `hybrid`, `native`, `apps`. Native mode pre-checks `pg_isready`/TCP 5432, Redis `PING`, `ffmpeg -version`, and `vips --version`, and forces `STORAGE_DRIVER=fs`. Wire up the root scripts listed in `RUNNING.md §3` (stub the app commands until the apps exist).
  - Verify: `bun run dev:hybrid` starts infra and prints "apps not yet scaffolded" for missing apps without crashing.

## Phase 1: Backend core (Go) 🔒

Use *superpowers:test-driven-development* (or *tdd*) for every task in this phase.

- [x] **1.1 Go module, config, logging**
  - Do: `apps/server/go.mod` (Go 1.26; the 2026 ecosystem — go-redis, sqlc — requires it). `internal/config` uses `caarlos0/env` and validates on start, and the error lists every missing var. Logging uses slog JSON. `cmd/api`, `cmd/worker`, and `cmd/tepatictl` each print their version and exit cleanly on SIGTERM. Pin goose, sqlc, and oapi-codegen as `tool` directives in a separate module `apps/server/tools/go.mod` (keeps the server module's dependency graph small; run them with `bun run go:tool <name>`).
  - Verify: `cd apps/server && go build ./... && go vet ./...`

- [x] **1.2 Migrations: initial schema**
  - Read: `ARCHITECTURE.md §4` (the DDL is the reference) and `SPEC.md §3, §5, §6`.
  - Do: Write the goose migrations for the schema, including the append-only trigger on `ledger_entries` and the extensions. Add `bun run db:migrate` / `db:rollback` / `db:new`.
  - Done when: up, down, and up again works on a fresh DB, and `UPDATE ledger_entries` raises an error.
  - Verify: `bun run db:migrate && bun run db:rollback -- --all && bun run db:migrate`, then `psql -c "update ledger_entries set amount=0"` fails.

- [x] **1.3 Domain: terms, deadlines, check-in FSM, ledger math** 🔒
  - Read: `SPEC.md §4–§7` completely.
  - Do: `internal/domain`, pure Go with no imports from store or http:
    - `Terms` struct with `Validate()` and `Hash()` (canonical JSON with sorted keys, then sha256).
    - `Deadlines(terms, member, localDate) (cutoffAt, submitDeadline time.Time)` and helpers for review, dispute, resolution, and override deadlines.
    - `Transition(ci CheckIn, ev Event, now time.Time, ctx TransitionCtx) (CheckIn, []Effect, error)`, where effects are `ApplyPenalty`, `ApplyReversal`, `Notify(kind)`, and `Decision(action)`.
    - `ClampPenalty(balance, penalty, floor) int64` and `ClampBonus(balance, bonus, cap *int64) int64`.
    - `ScheduledDates(terms, member) []civil.Date`.
  - Done when: table tests cover every arrow in the SPEC §5 diagram, every forbidden transition (returns `ErrInvalidTransition`), deadlines in `Asia/Jakarta`, `Asia/Makassar`, `Asia/Jayapura`, and `UTC`, the clamp at the floor and cap (including clamp-to-0), and override limits. Coverage ≥ 95% for `internal/domain`.
  - Verify: `go test ./internal/domain/... -cover`

- [x] **1.4 Store layer (sqlc)**
  - Do: `sqlc.yaml` (pgx/v5, UUIDs as `github.com/google/uuid`, `emit_interface: true`). Queries go in `db/queries/*.sql` for users, sessions (none: sessions are Redis), pacts, members, invites, check-ins (including batch generation and sweep selects with `FOR UPDATE SKIP LOCKED LIMIT $n`), proofs, attachments, decisions, ledger (insert, balance, and a passbook page with a running balance using `sum(amount) over (order by id)`), payouts, notifications, and outbox. Add a `store.WithTx` helper.
  - Verify: `bun run codegen && go build ./...`

- [x] **1.5 Services: pact lifecycle** 🔒
  - Do: `internal/service/pacts.go`: `CreateDraft`, `UpdateTerms` (bumps the version and clears acceptances), `Propose` (invite token, stored hashed), `Accept(termsHash, signatureName)`. When both accept, it moves the pact to `scheduled`, inserts the `pot_initial` entry, and generates check-ins for all scheduled dates with deadlines precomputed. `ActivateDuePacts(now)` handles the scheduled→active step.
  - Done when: integration tests (`internal/testdb`, real Postgres) show that accepting a stale hash fails with `pact.terms_mismatch`, a double accept is idempotent, and check-in counts equal the scheduled dates.
  - Verify: `go test -tags=integration ./internal/service/...`

- [x] **1.6 Services: check-in transitions and settlement** 🔒
  - Do: `SubmitProof`, `DeclareRest`, `Approve`, `Reject`, `Override`, `Dispute`, `ResolveDispute`, and `SweepDeadlines(now, batch)`. Each one runs a single tx: lock the check-in (expected status), call `domain.Transition`, apply effects (ledger insert with idempotency key, under the pact row lock for clamps), insert the decision row, and insert outbox notifications. `ClosePacts(now)` handles settling and the payout entry.
  - Done when the integration tests show:
    - Two concurrent `SweepDeadlines` calls produce exactly one penalty per check-in.
    - A penalised check-in that ends up approved writes a `reversal` (the FSM never penalises before a dispute resolves, so the test crafts that state directly; an upheld dispute itself moves no coins).
    - The pot never goes below the floor and never above the cap.
    - Overrides beyond `max_overrides` fail.
    - The worked example in `SPEC.md` (30-day pact, 4 misses at 50 coins, 1 backer miss) ends with the expected balance.
  - Verify: `go test -race -tags=integration ./internal/service/...`

- [x] **1.7 Auth** ⇄ (with 1.5/1.6 once 1.4 is done)
  - Do: Register and login (argon2id), Redis sessions (hashed token), logout, the `RequireUser` middleware, CSRF double-submit, and a login rate limit.
  - Verify: `go test ./internal/auth/... && go test -tags=integration ./internal/auth/...`

## Phase 2: API contract and HTTP

- [x] **2.1 OpenAPI contract** (fd303f0)
  - Read: `ARCHITECTURE.md §5`, and `SPEC.md` for field semantics.
  - Do: Write `api/openapi.yaml` (3.1) covering every endpoint in §5, with `problem+json` errors and stable `code` enums, the `Idempotency-Key` header parameter on mutating endpoints, and cursor pagination. Add `bun run codegen`, which generates Go (strict server) and TS (`apps/web/src/lib/api/schema.d.ts`).
  - Done when: `redocly lint api/openapi.yaml` (via `bunx @redocly/cli`) passes and the codegen output compiles.
  - Verify: `bunx @redocly/cli lint api/openapi.yaml && bun run codegen && cd apps/server && go build ./...`

- [x] **2.2 Fiber app and middleware** (3205c30)
  - Do: Middleware in the order given in `ARCHITECTURE.md §3`, with the Redis rate-limit storage, idempotency middleware (key = user + route + header; stores status and body for 24 h; replays the stored response), the problem+json error handler, `/healthz`, `/readyz`, and `/metrics`.
  - Verify: `go test ./internal/http/...` (includes the idempotency replay test and the rate-limit 429 test).

- [x] **2.3 Handlers** (b563a23..a780ff9) (implement the generated strict-server interface) ⇄ split by resource: auth/me, pacts/invites, check-ins/review, ledger/payout, notifications.
  - Done when: every operation in the YAML is implemented, and every handler test asserts the authorisation rule (a non-member gets 404, not 403, so pact existence doesn't leak).
  - Verify: `go test ./internal/http/... && bun run lint`, plus `go test -race -tags=integration ./internal/http/... ./internal/service/...` (the handler tests need Postgres and Redis, so they carry the `integration` tag).
  - Deviations, decided while building: (1) the three upload operations (`createUpload`, `completeUpload`, `getAttachment`) are routed but answer `503 server.unavailable` until 4.1/4.2 add the BlobStore and the media worker, so "every operation implemented" holds for the other 28; `TestEveryContractOperationIsRouted` still proves all 31 are routed. (2) Added `POST /invites/{token}/join`, which `ARCHITECTURE §5` lacked. Without it an invitee could not become a member, and accepting needs membership.

## Phase 3: Worker

- [x] **3.1 asynq server, scheduler, and outbox relay** 🔒 (ab82852..736cbcc)
  - Do: In `cmd/worker`, set up queues `critical`/`default`/`media` (weights 6/3/1) with periodic tasks `settlement:sweep` (every 1 min), `pacts:activate` (every 1 min), `pacts:close` (every 5 min), `outbox:relay` (every 5 s), `uploads:gc` (hourly), and `reminders:cutoff` (every 5 min, which enqueues unique reminder tasks for 3 h and 30 min before cutoff). Shut down gracefully.
  - Also: add `apps/server/.air.worker.toml` and `.air.api.toml` (`scripts/dev.ts` starts a process only when its air file exists, so `dev:hybrid` runs neither today), and `tepatictl seed` / `pact show` (see `docs/STATUS.md §7`).
  - Verify: `go test -tags=integration ./internal/jobs/...`. Then run `bun run dev:hybrid`, use `tepatictl seed --scenario overdue` to create a pact with an overdue check-in, and confirm it becomes `missed` with a ledger row within 2 minutes (inspect via `tepatictl pact show <id>`).
  - Result: verified live on 2026-10-07. `bun run db:seed` made an active pact with 3 overdue days; the worker marked all 6 check-ins `missed` within 35 s, wrote the `doer_miss`/`backer_miss` ledger rows, and relayed the `day_missed` notifications. `go test -race -tags=integration ./...` green.
  - Deviations, decided while building: (1) `reminders:cutoff` calls `service.SendReminders` directly instead of enqueueing a unique task per reminder. Dedupe is a new table `reminders_sent (check_in_id, kind)` written in the same transaction as the outbox row, which is stronger than asynq's TTL-based uniqueness and survives a Redis flush. It also sends the reviewer's 2 h `review_deadline_soon` reminder from SPEC §9. (2) The schedule uses `asynq.Scheduler`, not `PeriodicTaskManager` (the table is fixed; ADR-0004 updated). (3) `uploads:gc` is scheduled and handled, but does nothing until the BlobStore exists (4.1/4.2). (4) The `media` concurrency cap of 2 cannot be set per queue in asynq; it will be a semaphore in the `media:process` handler (4.2). (5) `RelayOutbox` only writes notifications. It returns the ones it delivered so 3.2 can enqueue emails after the commit.

- [x] **3.2 Notifications and email** ⇄ (ed2eca4..8a61460)
  - Do: In-app notifications, plus SMTP email (Mailpit in dev) using Indonesian templates (`html/template`, plain text fallback) for the triggers in `SPEC.md §9`.
  - Verify: Mailpit UI at `http://localhost:8025` shows the invite email after `tepatictl seed --scenario invite`.
  - Result: verified live on 2026-10-07. With `bun run dev:hybrid` running, `bun run db:seed -- invite` put an email to `seed-invitee@tepati.test` in Mailpit within 15 s: Indonesian subject, plain text and HTML parts, and the `/invite/<token>` link. `go test -race -tags=integration ./...` green.
  - Deviations, decided while building: (1) `Service.Propose` now writes an outbox row (topic `invite_mail`) when it is given an address; that row carries the plaintext invite token, and `MarkOutboxDispatched` strips `token` from the payload when the relay dispatches it, so only the hash outlives a few seconds (ADR-0004). (2) Dedupe is a claim column, `emailed_at` on `notifications` and `pact_invites` (migration `20261009000001`): the email task claims the row, sends, and releases the claim if the send fails. A crash between claim and send loses one email instead of sending two. (3) `proof_submitted` is a digest: tasks are unique per (user, pact, kind) and run 5 minutes after the first one, then one email covers everything unsent. (4) Email kinds are exactly the SPEC §9 "email" column (`internal/notify/delivery.go`); `rejection_final`, `dispute_upheld`/`dismissed` and `payout_*` stay in-app. (5) Copy uses the SPEC §1 glossary (kontrak, penyokong, pelaku, sanggahan). (6) SMTP uses the standard library (`net/smtp` driven by hand for context deadlines, STARTTLS when offered), no new dependency.

## Phase 4: Uploads and media

- [x] **4.1 BlobStore drivers** (2ee6dce..de7c8b6)
  - Do: Implement the `storage.BlobStore` interface (`PresignPut`, `PresignGet`, `Head`, `Get`, `Put`, `Delete`) with an `s3` driver (aws-sdk-go-v2, path-style, separate public endpoint for signing) and an `fs` driver (HMAC-signed local URLs served by `/api/v1/blob/*`).
  - Done when: integration test `TestPresignedPutRejectsWrongLength` runs against Garage, and its result is recorded in `docs/adr/0005` (append a note). If Garage does not enforce the signed length, implement `UPLOAD_MODE=proxy` and make it the default.
  - Verify: `go test -tags=integration ./internal/storage/...`
  - Result: verified 2026-10-07 against the local Garage. `TestPresignedPutRejectsWrongLength` passes (Garage answers `403` to a longer and to a shorter body), and a mutation check (presigning without the length) makes it fail with a `200`, so the test does detect the problem. **Garage enforces the signed length**, so `UPLOAD_MODE=presigned` stays the default and the proxy mode is not built. Recorded in ADR-0005.
  - Deviations, decided while building: (1) One contract suite (`contract_test.go`) runs against both drivers, so the fs driver cannot drift from the s3 one. It also checks content type, expiry, bucket separation and key validation. (2) `fs` also needed the HTTP side: `FS.ServeHTTP` verifies the HMAC and is mounted by the API at `/api/v1/blob/*` (before the session gate; the signature is the credential). Fiber has one body limit per app, so with the fs driver it is raised to the largest upload limit and JSON routes are re-capped at 1 MB by `jsonSizeCap`. (3) Keys are restricted to `[A-Za-z0-9._-]` segments joined by `/` (`ValidKey`), which is the path-traversal defence for fs. (4) The s3 driver turns off the SDK's default CRC32 checksums (`WhenRequired`) so presigned URLs need no header the browser cannot compute, and spools non-seekable readers to a temp file because signing over plain HTTP needs a seekable body. (5) `Delete` of a missing object is not an error, for both drivers.

- [x] **4.2 Upload intent, complete, and processing** 🔒 (limits) (ac7b781..825df97)
  - Do: Implement the endpoints and the `media:process` handler as specified in `ARCHITECTURE.md §6` and `SPEC.md §8`. Use ffmpeg and vips through `exec.CommandContext` with arg slices and timeouts, sniff types, enforce limits, and add the queue backpressure 503.
  - Done when: golden tests in `internal/media/testdata` pass:
    - A 12 MP JPEG with GPS becomes a WebP ≤ 2048 px with no EXIF.
    - A PNG renamed `.mp4` is rejected.
    - A 200-second video is rejected.
    - A 30-second 1080p video becomes 720p.
  - Verify: `go test ./internal/media/... && go test -tags=integration ./internal/service/... -run Upload`
  - Result: verified 2026-10-08 with ffmpeg 9.0.2 and libvips 8.18.7 on Windows. The four golden tests pass, and a mutation check (saving the WebP without `strip`) makes the EXIF test fail. `TestUploadedPhotoBecomesReadyThroughGarageAndTheWorker` runs the whole path with nothing faked: presigned PUT to Garage, `completeUpload`, the worker's media server, a signed GET of the WebP. `go test -race -tags=integration ./...` green.
  - Deviations, decided while building: (1) The golden inputs are generated by the tests (ffmpeg `testsrc2`, a hand-built GPS EXIF block) instead of committed to `internal/media/testdata`, which would add a 12 MP JPEG and a 1080p clip to the repo. The tests skip with a message when the tools are missing. (2) The `media` concurrency cap of 2 is a **second asynq server** that serves only the media queue (`Options.MediaWorkers`, default 2), not a semaphore in the handler as ADR-0004 planned: a semaphore would park waiting tasks on the shared pool and starve settlement. ADR-0004 is updated. (3) Added `GiveUpAttachment`: when `media:process` has used its retries (`MaxRetry 3`), the attachment becomes `rejected` with a reason instead of staying `processing`. (4) `CreateAttachment` and `MarkAttachmentReady` take their timestamps from the injected clock (invariant 4). (5) Added `UPLOAD_PACT_QUOTA_BYTES` (default 1 GB) and `VIPSHEADER_PATH` (image sizes without decoding; ships with libvips). (6) `uploads:gc` also queues `media:process` again for uploads still `uploaded` after 5 minutes, so a lost enqueue (Redis down at `completeUpload`) heals. (7) The handler rejects `bytes < 1` itself because the generated server does not enforce schema minimums. (8) `dev:native` now also checks `ffprobe` and `vipsheader`.

## Phase 5: Web foundation

Before any UI code: load *impeccable* (it runs `impeccable context`, which loads PRODUCT.md and the surface brief), then read `~/.claude/skills/impeccable/reference/craft-floor.md`. Use *taste-skill* only as a pre-flight checklist (sections 3–4 and 9). Its landing-page rules don't apply to the app shell. Use context7 for current Next.js 16, Tailwind v4, Tiptap 3, and next-intl APIs.

- [x] **5.1 Next.js app on Bun** (21e40b4..b671f19)
  - Do: Create `apps/web` (TS strict, App Router, `src/`), `bun --bun next dev`, Tailwind v4 (`@tailwindcss/postcss`), ESLint, `next/font` with Geist and Geist Mono, next-intl (default `id`, no URL prefix), TanStack Query provider, and an `openapi-fetch` client with CSRF and Idempotency-Key helpers. Server-side fetches use `API_INTERNAL_URL` and forward cookies. Dev rewrites send `/api/*` to `http://localhost:8080`.
  - Verify: `cd apps/web && bun run build && bun run typecheck`
  - Result: verified 2026-10-08 with Next 16.4.0 (Turbopack), React 19.3, Tailwind 4.3, next-intl 4.14, Bun 1.4.2. `bun run build` and `bun run typecheck` pass, `eslint` is clean, and the 20 unit tests in `src/lib/api/api.test.ts` pass (written first and watched fail). Live, through the Next dev server on :3000 against the real API: `/` renders with `lang="id"`, `/api/v1/me` is proxied and answers `401 application/problem+json`, and the client helpers registered a user, got `403 auth.csrf` for a POST without the CSRF header and `204` with it. `bun run lint`, `bun run test` and `bun run codegen --check` are green.
  - Deviations, decided while building: (1) Fonts come from the `geist` package (self-hosted, no network at build time) instead of `next/font/google`; they are still loaded through `next/font`. (2) TypeScript stays on 5.9 (7.x exists on npm; Next's tooling is not verified against it), ESLint on 9, `@types/node` on 22 (the machine runs Node 20). (3) `unwrap()` was added: openapi-fetch has already read the error body when it resolves, so `ApiError.fromResponse` cannot be applied to its result (found by the live check). (4) The `Idempotency-Key` is generated by the client only when the caller gave none; a caller that retries one action passes its own key so the server can replay. ARCHITECTURE §7 said "per attempt", which would defeat idempotency on a retry. (5) `output: "standalone"` is set now for the Docker image in 7.1. (6) `codegen --check` looked at the whole `apps/web/src/lib/api` folder, which now also holds the hand-written client, so it checks `schema.d.ts` only. (7) `next dev` writes `apps/web/AGENTS.md` (Next's agent rules pointing at `node_modules/next/dist/docs/`); it is committed because the tool re-creates it otherwise. (8) The page is a placeholder, so there are no screenshots; `impeccable detect` over the three changed UI files reported nothing. Tokens and the real shell come in 5.2 and 5.3.

- [x] **5.2 Design tokens and primitives (from the direction contract)** (05047de..6008608)
  - Read: `.impeccable/surfaces/apps-web-src-app-app.md` (the whole contract). Hex values are given there. Translate them into OKLCH tokens in `src/styles/tokens.css`, with a light theme and a dark "desk lamp" theme. The dark theme must be designed, not just inverted.
  - Do: Add the owned shadcn/ui primitives (button, input, textarea, dialog, sheet, tabs, select, toast, tooltip, badge) and restyle them to the contract: radius 6/10, cover-teal primary, and stamp violet reserved for decision buttons. Add `Amount` (mono, sign, D/K label, IDR equivalent), `Countdown` (fixed-cell mono digits, `aria-live="polite"` once per minute, not every second), `StatusChip` (one per check-in status, icon plus label plus colour), and `MemberLine` (fixed member colour). Icons come from Phosphor only.
  - Done when: a `/dev/kitchen-sink` page (dev builds only) shows every primitive in both themes and at 390/1440 px.
  - Verify: `bun run build`, then *playwright* screenshots of `/dev/kitchen-sink` at 390 and 1440 in light and dark. Look at all four.
  - Result: verified 2026-10-08. `bun run build`, `bun run lint` and `bun run test` pass (154 web unit tests, Go tests unchanged). Screenshots of `/dev/kitchen-sink` were taken with Playwright at 390 and 1440 px in light and dark (390 light and dark, 1440 light and dark, plus the open dialog, bottom sheet and error toast) and looked at; they are not committed. `impeccable detect` over `components`, `app` and `styles` reported nothing. A production build answers `404` for `/dev/kitchen-sink`. `tokens.test.ts` parses `tokens.css` and checks 35 text and shape pairs against WCAG AA in both themes; it caught a dark stamp hover at 3.7:1 before any screenshot.
  - Found while looking, and fixed: (1) next-intl rejects a `.` in a message key, so the `Errors.auth.unauthenticated`-style keys from 5.1 threw `INVALID_KEY` at runtime in dev (the build and the 5.1 tests did not notice). Keys are now `auth_unauthenticated` (`errorMessageKey` maps the code), and a test fails on any dotted key. (2) The modal scrim used `ink`, which is light in the dark theme and washed the page out; it is now its own `scrim` token. (3) `Amount`'s unit, D/K mark and Rupiah line were `em`-based and fell under 11 px at the small step; they use fixed sizes now.
  - Deviations, decided while building: (1) Colours are `light-dark(light, dark)` pairs under `color-scheme` instead of a `[data-theme]` block per theme, so each token is declared once and the tests can read both sides. The theme follows the system, or the `tepati_theme` cookie (`system`, `light`, `dark`) that the root layout turns into `data-theme`; a `ThemeToggle` sets it. (2) The default Tailwind palette, radii and shadows are removed (`--color-*: initial`, and so on), so a stray `bg-red-500` or `rounded-xl` does not build; the shape lock (6/10) is enforced by the tool. (3) Primitives are on the `radix-ui` package with `class-variance-authority` and a `cn` that knows the token names; there is no shadcn CLI output, the files are written by hand to the contract. (4) Extra tokens beyond the contract's hex list, all derived from it and tested: surface, sunken, muted, placeholder, rule-strong, tints, `stamp-text`, `teal-text`, `today-ink`, member foregrounds, `ring`, `scrim`. (5) `approved` and `rejected` chips wear the stamp violet because they show the outcome of a human decision; `auto_approved` does not. Each status has its own icon and label. (6) The `Countdown` takes an optional `serverNow` and lines the phone clock up with it; without it the first paint is `--:--:--`, never a wrong time. (7) `guilloche` is a CSS utility (two hairline radial rings); it is shown on the demo header band and meant for the pot header band only. (8) Passbook print (the signature move) is not built here; it belongs to 6.4. (9) The kitchen-sink copy is hard-coded demo data because it is dev-only; the primitives themselves take all their copy from `messages/*.json`. (10) Written test-last for the components (the logic in `format`, `countdown`, `member` and the tokens was test-first).

- [x] **5.3 Auth pages and app shell** (a9f3db3..884e87d)
  - Do: `/login`, `/register`, and the `(app)` layout. Desktop has a cover-teal left rail (Hari ini, Kontrak, Tinjau, Pengaturan, notification bell). Mobile has a bottom tab bar with the same four items. Add auth-guard middleware.
  - Verify: Playwright test `auth.spec.ts` (register → lands on /today → logout).
  - Result: verified 2026-10-08. `bun run lint`, `bun run test` (243 web unit tests, up from 154; 11 script tests; Go unchanged) and `bun run build` pass, and the build lists `Proxy (Middleware)`. `bun run test:e2e` ran `auth.spec.ts` against the real API, Postgres and Redis: 6 passed, 2 skipped on purpose (desktop 1440 and mobile 390 projects). The specs cover: register lands on `/today`, the right navigation for the screen size (rail or tab bar, same four items, `aria-current`), logout returns to `/login` and a private page then bounces to `/login?next=%2Ftoday`, a private page asked for while signed out is reached after login, wrong credentials show a form-level error and stay on `/login`, a short password is caught in the browser with focus on the field. Screenshots at 390 and 1440 px in light and dark (login, register with errors, Hari ini, Kontrak, Pengaturan, and a failed login) were taken and looked at; they are not committed. `impeccable detect` over the 18 changed UI files reported nothing. Live: `/`, `/today`, `/review` without a session answer 307 to `/login` (with `next` for the private pages).
  - Found while building, and fixed: (1) On the first dev start the proxy was not registered at all (empty middleware manifest, no redirects, only the layout's own redirect worked); it happened while `bun add` was rewriting `node_modules`, and a restart fixed it. A cold start afterwards was fine and the dev log shows `proxy.ts: Nms` per request, so look for that when a guard seems to do nothing. (2) A `matcher` regex written as `"\\."` in a TS string must keep both backslashes; one backslash is eaten by the string and the proxy then matches nothing. (3) The generated `RegisterRequest` makes `timezone` required (it has a default), so the form sends the browser's zone or `Asia/Jakarta`. (4) In the e2e specs `getByRole("alert")` also matches Next's route announcer, so alert locators are scoped to the form.
  - Deviations, decided while building: (1) The proxy does not bounce a signed-in visitor off `/login`: a stale cookie would loop (`/login` -> `/today` -> layout 401 -> `/login`). The login and register pages ask the API (`/me`) and redirect themselves. The `(app)` layout redirects to a plain `/login` on 401 because it does not know the path; the proxy adds `next` for the no-cookie case. (2) The bell is an inert unread-count readout (PLAN allows it) in the desktop rail only. On a phone there is no bell yet; the 6.2 header band is its place. (3) `/today`, `/pacts` and `/review` are placeholders with honest empty states. `/settings` is real (name, email, time zone, theme toggle, logout). (4) Logout is in the rail footer on desktop and in Pengaturan on a phone. (5) A show/hide password switch was added to the forms (not asked for). (6) The forms rely on the client's per-request `Idempotency-Key`; login and register have no such header in the contract. (7) The API allows 10 auth requests a minute per IP and the limit is not configurable, so the e2e keeps to about 7 per full run (desktop runs four specs, mobile two) and a second full run needs a minute's wait. `bun run test:e2e` does not start the stack; it checks that web and API answer and then runs Playwright. (8) The `Home` message namespace went away with the placeholder page. (9) Unit tests for guard, forms and nav were written first and watched fail; components and pages are covered by the e2e and the screenshots, not by unit tests.

## Phase 6: Web features

Each task's Done when includes: loading, empty, and error states; Indonesian copy in `messages/id.json` (English stubs in `en.json`); keyboard access; 390 px and 1440 px layouts.

- [x] **6.1 New pact wizard and invite/accept flow** 🔒 (terms shown to users) (632b9cd..2a94628)
  - Do: `/pacts/new` has steps Basics → Komitmen (per member) → Koin & aturan → Tinjau ketentuan. It shows the plain-language terms summary from `SPEC.md §4`, then Propose, which gives a copyable invite link. `/invite/[token]` lets the invitee review the terms, sign by typing their name, and accept. When the terms change, both signatures are visibly cleared.
  - Verify: Playwright `pact-create.spec.ts` with two browser contexts (A creates, B accepts) ends with the pact `scheduled`.
  - Result: verified 2026-10-08. `bun run lint`, `bun run test` (341 web unit tests, up from 243, 98 of them new in `src/lib/pact`; 11 script tests; Go unchanged) and `bun run build` pass, and the build lists `/pacts/new`, `/pacts/[id]`, `/pacts/[id]/edit` and `/invite/[token]`. `bun run test:e2e` ran against the real API, Postgres and Redis: 7 passed, 3 skipped on purpose (the mobile project skips the specs that cost auth requests). `pact-create.spec.ts` follows A and B through the whole flow with two browser contexts: A walks the four steps (empty name refused with focus on the field, empty commitments refused), sees the terms in plain language ("Kalau Pelaku melewatkan 1 hari, pot berkurang 50 koin (Rp50.000).") and proposes; B opens the link signed out, is sent to `/login?next=/invite/<token>`, registers and lands back on the invite, is refused without the tick and with a wrong name, signs with a differently-cased name and ends on the pact page; the same link then says it cannot be used; A edits the pot, and both signatures show as cleared on both screens ("Belum menandatangani" twice, plus the note); both sign again and the pact is `Terjadwal`. Live check with curl: a pact whose backer does not commit (`backer_commits: false`, empty commitment and schedule) is accepted by the API (201). Screenshots at 390 and 1440 px in light and dark (every wizard step with and without errors, the share screen, the invite, the pact page waiting, half-signed and scheduled, the list) were taken and looked at; they are not committed. `impeccable detect` over the changed UI reported nothing.
  - Found while building: (1) `joinInvite` re-keys the doer slot, which is a terms change: a pact is at `terms_version` 2 right after the invitee joins and at 3 after one edit. So the page never prints a version number, and the "terms changed" note also appears for a doer who joined but did not finish signing (the join did clear the signatures, so the note is true). (2) The server does not look at `starts_on` against today: a pact that starts in the past is accepted and would generate check-ins that are already overdue. The wizard only allows tomorrow or later. A pact proposed for tomorrow and signed after tomorrow has the same problem and is not handled (see `docs/STATUS.md`, known gaps). (3) `problem.errors[]` is still never populated, so a server `pact.invalid_terms` can only be shown as one general message; the wizard mirrors every rule of `Terms.Validate` so the server should rarely be the one to say no. (4) Port 8080 was taken by another project's container on the owner's machine, so the stack ran with `API_PORT` and `API_INTERNAL_URL` set to 18080 in the (gitignored) `.env`, and the e2e needs `E2E_API_URL=http://localhost:18080`. (5) If `acceptPact` fails after `joinInvite` succeeded, the invitee is already a member, so the invite page sends them to the pact page with a toast and the sign form is there.
  - Deviations, decided while building: (1) A pact may start tomorrow or later in its own time zone, not today (SPEC is silent; this is the choice that moves no coins, see Found 2). (2) A member's schedule must contain at least one date between the start and end, otherwise the step is refused (SPEC is silent; the server would accept it and create no check-ins). (3) A backer who does not commit sends `commitment: ""`, an empty schedule and `penalty_per_miss: 1`; the contract says commitment has `minLength: 1`, the server ignores the member's rules in this case and the live check passed. (4) Either member can edit the terms before both have signed (SPEC §3), through the same wizard at `/pacts/[id]/edit`; the ids come from the pact, not from who is looking. (5) `/pacts/[id]` is an agreement view only (signatures, terms, sign form, invite tools, edit). The passbook and calendar are 6.4; `active`, `settling` and `completed` pacts show a one-line note. (6) The coin rate (1 coin = Rp1.000) and the pot floor (0) are not on the form; they are kept as they are on edit. (7) The invitee reads the summary with their own name in it, because it is also the name they type to sign; the not-yet-joined doer is called "Pelaku". (8) Joining and signing are one button on the invite page and two calls. (9) The review step has an optional email for the invite (it uses `ProposeRequest.email`); without it the backer copies the link. (10) A minimal `/pacts` list was built, because the wizard needs an entry point. (11) The new-pact e2e runs on the desktop project only and registers two users, which brings a full `bun run test:e2e` to 9 of the 10 auth requests a minute; wait a minute between full runs. (12) Unit tests for dates, draft, summary and agreement were written first and watched fail; components and pages are covered by the e2e and the screenshots.

- [x] **6.2 Today screen** (first viewport defined in the direction contract) (01646bd..de149b3)
  - Do: Countdown header, Hari ini panel (submit / rest day / status), Perlu ditinjau rows, and a mini passbook. Multiple active pacts stack as sections ordered by the nearest deadline.
  - Verify: Playwright screenshot review at 390 and 1440, plus `today.spec.ts` with states open, submitted, approved, and missed (use the test clock).
  - Result: verified 2026-10-08. `bun run lint` (typecheck, eslint, gofmt, vet), `bun run test` (359 web unit tests, up from 341; 13 script tests; Go unit tests) and `bun run build` pass. `go test -tags=integration ./internal/ctl` passes against real Postgres (the new seed). A full `bun run test:e2e` against the real API, Postgres, Redis and worker: 11 passed, 3 skipped on purpose. `today.spec.ts` runs on both projects and checks: the four sections come in nearest-deadline order (missed, open, then the two with tomorrow's deadline); each shows its commitment and its chip (Terlewat, Terbuka, Disetujui, Menunggu tinjauan); the open one has a countdown and "Batas 23:59, masa tenggang sampai 00:29" and a disabled "Kirim bukti"; the submitted one says who reviews and by when and has 5 words; the missed pact's mini passbook has one debit line of 50 and a saldo of 950; "Perlu ditinjau (1)" links to `/review`; the bell is in the band on a phone. A second spec confirms a rest day in a dialog (cancelling changes nothing), then the chip, the text and the toast change, the button goes and the band stops counting to that pact. Screenshots at 390 and 1440 px in light and dark (Today, the rest dialog, the loading skeleton) were taken and looked at; they are not committed. `impeccable detect` over the changed UI reported nothing.
  - Test clock: decided with the owner (2026-10-08): no endpoint. `tepatictl seed --scenario today` (`ctl.SeedToday`) builds four active pacts for one new doer through the real service with a `FakeClock` (create, propose, join, both sign, activate, submit, approve) and then runs the real `SweepDeadlines`; the missed pact has a midnight cutoff and no grace, so today's check-in is already overdue. Users are new on every run (`today-<tag>-doer@tepati.test`), so the 10-open-pacts limit is never reached. `bun run test:e2e` seeds one doer per Playwright project and passes the logins as `E2E_TODAY_EMAIL_DESKTOP`, `E2E_TODAY_EMAIL_MOBILE` and `E2E_TODAY_PASSWORD`; the spec skips itself when they are absent. The auth rate limit became `AUTH_RATE_LIMIT_PER_MIN` (default 10; validation refuses more than 10 in production), set to 200 in the local `.env` so the whole suite runs at once.
  - Found while building: (1) `getToday` has `review_queue_count` but no rows, so the screen also calls `getReviewQueue` (limit 5); and `TodayCheckIn` carries no commitment and no rest-day count, so the page loads `getPact` for each active pact (a handful at most). A richer `getToday` would save those calls (STATUS, known gaps). (2) A today-dated check-in that is already `missed` stays in `my_check_ins`, which is what the screen needs, but an *earlier* missed one does not, so a missed yesterday is visible only as a debit line in the passbook. (3) `id-ID` prints 23.59 with a dot, while the terms say 23:59; times now go through `clockIn` (`en-GB`, `h23`) so every screen prints a colon. (4) `<header>` inside `<main>` has no `banner` role, so specs find the band with `main > header`. (5) The first screenshot caught the `loading.tsx` skeleton (it works); take screenshots after the h1 appears.
  - Deviations, decided while building: (1) "Kirim bukti" is a disabled button with a sentence under it that sending proof arrives in the next update (task 6.3); a link would lead to a page that does not exist. Rest days are fully working because `declareRest` exists. (2) The band counts to the soonest `submit_deadline` (cutoff plus grace) of an open check-in, the same instant the API calls `next_deadline`, and the panel prints both the cutoff and the end of the grace. (3) The review rows link to `/review` (still a placeholder) because the check-in detail page is 6.5. (4) The mini passbook has one signed "Jumlah" column with the D/K mark instead of separate debit and credit columns, so it fits 390 px; the full passbook (6.4) has them all. (5) The page greeting ("Halo, {name}.") is gone; the band carries the title and the date, and `auth.spec.ts` now expects the empty-state sentence. (6) Rest-day confirm is a dialog (a decision with a count that cannot be undone), in the primary colour, not stamp violet, because violet is for decisions about someone else's proof and signing. (7) `ledgerEntry`, `restLeft`, `groupToday`, `nextDeadline`, `dayLabel` and `clockIn` have unit tests written first; the components are covered by the e2e and the screenshots.

- [ ] **6.3 Proof editor and submission**
  - Do: A Tiptap 3 editor with the allowed nodes only (SPEC §8), a toolbar that fits on mobile, and a CharacterCount and word count against the `min_words` rule. The attachment tray supports camera capture, drag-drop, and paste. Images are compressed in the browser first. Each upload shows progress → processing → ready/rejected with the reason. The submit button stays disabled until the evidence rules pass and every attachment is ready.
  - Verify: Playwright `proof.spec.ts` uploads a fixture image and video and submits. Also verify that a 250 MB fake file is rejected client-side, and that a renamed file is rejected server-side with a visible reason.

- [ ] **6.4 Pact page: passbook and calendar** (signature move)
  - Do: `/pacts/[id]` shows a header (members with line colours, terms summary link, status), the **passbook** (date, keterangan, debit, kredit, saldo; infinite scroll with a cursor), and a month calendar with each check-in status as a symbol plus colour. The passbook-print motion runs when a new entry arrives (poll every 30 s, or on mutation success). Respect reduced motion.
  - Verify: Playwright scenario: miss a day via the test clock and watch the new D line appear with the saldo updated. Screenshot it.

- [ ] **6.5 Review queue and check-in detail** 🔒 (power visibility)
  - Do: `/review` lists submissions sorted by deadline, with deadline countdowns. `/pacts/[id]/days/[date]` shows the proof (rendered read-only with Tiptap's static renderer), version history, attachments (lightbox, video player), the decision timeline (every `decisions` row, with power actions in stamp violet), and the actions approve / reject (reason required) / override (shows the remaining overrides count) / dispute / resolve.
  - Verify: Playwright `review.spec.ts` covers approve, reject → dispute → uphold, and override (the doer sees "Dibatalkan oleh penyokong" with the reason).

- [ ] **6.6 Settlement and payout screens** ⇄
  - Do: Pact `settling` state: the final saldo with IDR equivalent, "Tandai sudah dibayar" (backer), and "Konfirmasi diterima" (doer). Completed state shows a summary.
  - Verify: Playwright `settlement.spec.ts`.

- [ ] **6.7 Notifications inbox and settings** ⇄
  - Verify: Playwright smoke test plus screenshots.

## Phase 7: Full Docker and deploy targets

- [ ] **7.1 Dockerfiles**
  - Do: `deploy/docker/server.Dockerfile` (multi-stage; build `api`, `worker`, and `tepatictl`; the runtime is `debian:bookworm-slim` with `ffmpeg libvips-tools ca-certificates tzdata`, user 10001). `deploy/docker/web.Dockerfile` (`oven/bun` build with Next `output: 'standalone'`, runs with `bun server.js`, user 10001).
  - Verify: `bun run deploy:build -- --env staging` and `docker image ls | grep tepati` (sizes recorded in the PR).

- [ ] **7.2 compose.dev.yaml and compose.prod.yaml, Caddy**
  - Read: `docs/RUNNING.md §4` again and follow every mount rule.
  - Do: Dev profile with hot reload (air polling, `WATCHPACK_POLLING`). Prod overrides: Caddyfile (same origin, `/api/*` → api, media under `/s3/*` or `media.` host, security headers, request body caps), a `migrate` one-shot dependency, resource limits, and a `backup` profile.
  - Verify: `bun run dev:docker` → `http://localhost:3000` works end to end. Then `bun run deploy:up -- --env staging` on a local machine with `localhost` domains → health checks are green → the Playwright smoke suite passes with `BASE_URL=https://localhost`.

- [ ] **7.3 Native mode polish**
  - Verify: Stop Docker, install Postgres, Redis (Memurai), ffmpeg, and vips locally, run `bun run dev:native`, and complete the proof upload flow with `STORAGE_DRIVER=fs`.

## Phase 8: Quality gates

- [ ] **8.1 E2E golden path in CI**: GitHub Actions running lint, unit, integration, and Playwright e2e against `dev:docker`.
- [ ] **8.2 impeccable finish**: run the finish review for the app shell surface, as `new-work.md §7` describes (screenshots in `.impeccable/review/`, detector run, `impeccable-finish-reviewer` subagent), fix in at most 2 rounds, then the documenter writes **`DESIGN.md` and `.impeccable/design.json`**.
- [ ] **8.3 web-design-guidelines audit**: run *web-design-guidelines* over `apps/web/src`. Fix the findings or record them as accepted.
- [ ] **8.4 Load test**: k6 script `tests/load/today.js` (200 RPS mixed reads plus 20 RPS submits for 5 minutes) against staging compose. Record p95 against the targets in `SPEC.md §10`.
- [ ] **8.5 Security review**: `/security-review` plus a manual checklist (authz on every pact-scoped query, upload hardening, CSP).

## Phase 9: Staging

- [ ] **9.1 Deploy to a VPS** (Docker + Compose), with DNS, TLS via Caddy, backups, and `docs/RUNBOOK.md` (restore drill done once).
- [ ] **9.2 Landing page (Persuade surface)**: run `/impeccable shape landing`, which gets its own surface brief and direction round.

---

### Worked example (used by test 1.6)

The pact runs 2026-11-02 → 2026-11-29 (4 weeks). Doer schedule: Mon–Sat (24 days), penalty 50, 2 rest days. Backer commits Mon–Fri (20 days), penalty 50. Initial pot 1000, floor 0, cap 1500.
- The doer misses 4 days, the backer misses 1 day, and the doer takes 2 rest days. One doer rejection is disputed and upheld (no penalty), and one auto-approval is overridden (penalty).
- Ledger: +1000 (initial) −4×50 (misses) −50 (override) +50 (backer miss) = **800**, then payout −800, so the final balance is 0 and 800 coins (≈ Rp800.000) is owed to the doer.
