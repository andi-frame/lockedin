# Tepati: Implementation Plan

This plan is written so that **any capable agent (for example Claude Sonnet 5.5) or human developer can pick up the next unchecked task and finish it without extra context**. Read `AGENTS.md` first.

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

- [ ] **2.2 Fiber app and middleware**
  - Do: Middleware in the order given in `ARCHITECTURE.md §3`, with the Redis rate-limit storage, idempotency middleware (key = user + route + header; stores status and body for 24 h; replays the stored response), the problem+json error handler, `/healthz`, `/readyz`, and `/metrics`.
  - Verify: `go test ./internal/http/...` (includes the idempotency replay test and the rate-limit 429 test).

- [ ] **2.3 Handlers** (implement the generated strict-server interface) ⇄ split by resource: auth/me, pacts/invites, check-ins/review, ledger/payout, notifications.
  - Done when: every operation in the YAML is implemented, and every handler test asserts the authorisation rule (a non-member gets 404, not 403, so pact existence doesn't leak).
  - Verify: `go test ./internal/http/... && bun run lint`

## Phase 3: Worker

- [ ] **3.1 asynq server, scheduler, and outbox relay** 🔒
  - Do: In `cmd/worker`, set up queues `critical`/`default`/`media` (weights 6/3/1) with periodic tasks `settlement:sweep` (every 1 min), `pacts:activate` (every 1 min), `pacts:close` (every 5 min), `outbox:relay` (every 5 s), `uploads:gc` (hourly), and `reminders:cutoff` (every 5 min, which enqueues unique reminder tasks for 3 h and 30 min before cutoff). Shut down gracefully.
  - Verify: `go test -tags=integration ./internal/jobs/...`. Then run `bun run dev:hybrid`, use `tepatictl seed --scenario overdue` to create a pact with an overdue check-in, and confirm it becomes `missed` with a ledger row within 2 minutes (inspect via `tepatictl pact show <id>`).

- [ ] **3.2 Notifications and email** ⇄
  - Do: In-app notifications, plus SMTP email (Mailpit in dev) using Indonesian templates (`html/template`, plain text fallback) for the triggers in `SPEC.md §9`.
  - Verify: Mailpit UI at `http://localhost:8025` shows the invite email after `tepatictl seed --scenario invite`.

## Phase 4: Uploads and media

- [ ] **4.1 BlobStore drivers**
  - Do: Implement the `storage.BlobStore` interface (`PresignPut`, `PresignGet`, `Head`, `Get`, `Put`, `Delete`) with an `s3` driver (aws-sdk-go-v2, path-style, separate public endpoint for signing) and an `fs` driver (HMAC-signed local URLs served by `/api/v1/blob/*`).
  - Done when: integration test `TestPresignedPutRejectsWrongLength` runs against Garage, and its result is recorded in `docs/adr/0005` (append a note). If Garage does not enforce the signed length, implement `UPLOAD_MODE=proxy` and make it the default.
  - Verify: `go test -tags=integration ./internal/storage/...`

- [ ] **4.2 Upload intent, complete, and processing** 🔒 (limits)
  - Do: Implement the endpoints and the `media:process` handler as specified in `ARCHITECTURE.md §6` and `SPEC.md §8`. Use ffmpeg and vips through `exec.CommandContext` with arg slices and timeouts, sniff types, enforce limits, and add the queue backpressure 503.
  - Done when: golden tests in `internal/media/testdata` pass:
    - A 12 MP JPEG with GPS becomes a WebP ≤ 2048 px with no EXIF.
    - A PNG renamed `.mp4` is rejected.
    - A 200-second video is rejected.
    - A 30-second 1080p video becomes 720p.
  - Verify: `go test ./internal/media/... && go test -tags=integration ./internal/service/... -run Upload`

## Phase 5: Web foundation

Before any UI code: load *impeccable* (it runs `impeccable context`, which loads PRODUCT.md and the surface brief), then read `~/.claude/skills/impeccable/reference/craft-floor.md`. Use *taste-skill* only as a pre-flight checklist (sections 3–4 and 9). Its landing-page rules don't apply to the app shell. Use context7 for current Next.js 16, Tailwind v4, Tiptap 3, and next-intl APIs.

- [ ] **5.1 Next.js app on Bun**
  - Do: Create `apps/web` (TS strict, App Router, `src/`), `bun --bun next dev`, Tailwind v4 (`@tailwindcss/postcss`), ESLint, `next/font` with Geist and Geist Mono, next-intl (default `id`, no URL prefix), TanStack Query provider, and an `openapi-fetch` client with CSRF and Idempotency-Key helpers. Server-side fetches use `API_INTERNAL_URL` and forward cookies. Dev rewrites send `/api/*` to `http://localhost:8080`.
  - Verify: `cd apps/web && bun run build && bun run typecheck`

- [ ] **5.2 Design tokens and primitives (from the direction contract)**
  - Read: `.impeccable/surfaces/apps-web-src-app-app.md` (the whole contract). Hex values are given there. Translate them into OKLCH tokens in `src/styles/tokens.css`, with a light theme and a dark "desk lamp" theme. The dark theme must be designed, not just inverted.
  - Do: Add the owned shadcn/ui primitives (button, input, textarea, dialog, sheet, tabs, select, toast, tooltip, badge) and restyle them to the contract: radius 6/10, cover-teal primary, and stamp violet reserved for decision buttons. Add `Amount` (mono, sign, D/K label, IDR equivalent), `Countdown` (fixed-cell mono digits, `aria-live="polite"` once per minute, not every second), `StatusChip` (one per check-in status, icon plus label plus colour), and `MemberLine` (fixed member colour). Icons come from Phosphor only.
  - Done when: a `/dev/kitchen-sink` page (dev builds only) shows every primitive in both themes and at 390/1440 px.
  - Verify: `bun run build`, then *playwright* screenshots of `/dev/kitchen-sink` at 390 and 1440 in light and dark. Look at all four.

- [ ] **5.3 Auth pages and app shell**
  - Do: `/login`, `/register`, and the `(app)` layout. Desktop has a cover-teal left rail (Hari ini, Kontrak, Tinjau, Pengaturan, notification bell). Mobile has a bottom tab bar with the same four items. Add auth-guard middleware.
  - Verify: Playwright test `auth.spec.ts` (register → lands on /today → logout).

## Phase 6: Web features

Each task's Done when includes: loading, empty, and error states; Indonesian copy in `messages/id.json` (English stubs in `en.json`); keyboard access; 390 px and 1440 px layouts.

- [ ] **6.1 New pact wizard and invite/accept flow** 🔒 (terms shown to users)
  - Do: `/pacts/new` has steps Basics → Komitmen (per member) → Koin & aturan → Tinjau ketentuan. It shows the plain-language terms summary from `SPEC.md §4`, then Propose, which gives a copyable invite link. `/invite/[token]` lets the invitee review the terms, sign by typing their name, and accept. When the terms change, both signatures are visibly cleared.
  - Verify: Playwright `pact-create.spec.ts` with two browser contexts (A creates, B accepts) ends with the pact `scheduled`.

- [ ] **6.2 Today screen** (first viewport defined in the direction contract)
  - Do: Countdown header, Hari ini panel (submit / rest day / status), Perlu ditinjau rows, and a mini passbook. Multiple active pacts stack as sections ordered by the nearest deadline.
  - Verify: Playwright screenshot review at 390 and 1440, plus `today.spec.ts` with states open, submitted, approved, and missed (use the test clock).

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
