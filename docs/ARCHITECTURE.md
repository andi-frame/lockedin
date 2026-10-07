# Tepati: Architecture

Audience: human developers and coding agents. Read `PRODUCT.md` (why) and `docs/SPEC.md` (behaviour) first. Decisions and their rationale live in `docs/adr/`. Diagrams live in `docs/diagrams/` (archify HTML; open in a browser).

## 1. System overview

```
                          ┌──────────────── Browser (PWA, mobile-first) ────────────────┐
                          │  Next.js app (RSC + client islands), Tiptap editor          │
                          └───────────────┬───────────────────────────┬──────────────────┘
                                          │ same origin               │ presigned PUT (direct)
                                ┌─────────▼─────────┐                 │
                                │   Caddy (edge)    │  TLS, gzip/zstd, routing, upload size cap
                                └───┬───────────┬───┘                 │
                        /*  (pages) │           │ /api/*              │
                     ┌──────────────▼──┐   ┌────▼──────────────┐      │
                     │ web (Next.js on │   │ api (Go + Fiber)  │──────┼──────────┐
                     │ Bun), N replicas│   │ stateless, N repl.│      │          │
                     └─────────────────┘   └──┬─────┬──────┬───┘      │          │
                                              │     │      │ enqueue  │          │
                                    ┌─────────▼─┐ ┌─▼──────▼──┐ ┌─────▼──────────▼───┐
                                    │ Postgres  │ │  Redis    │ │ Garage (S3 API)     │
                                    │ (truth)   │ │ cache,    │ │ buckets: staging,   │
                                    └─────▲─────┘ │ sessions, │ │ media               │
                                          │       │ ratelimit,│ └─────▲───────────────┘
                                          │       │ asynq     │       │
                                    ┌─────┴───────┴───▲───────┘       │
                                    │ worker (Go, asynq server):      │
                                    │ settlement sweep, media         ├─ ffmpeg, libvips (CLI)
                                    │ processing, notifications, email│
                                    └─────────────────────────────────┘
```

| Service | Tech | Scales | State |
|---|---|---|---|
| `edge` | Caddy 2 | 1 (prod) | certificates volume |
| `web` | Next.js (latest 16.x) on Bun ≥ 1.3.14, React 19, TS | horizontal | none |
| `api` | Go 1.26+, Fiber v3, pgx v5, sqlc | horizontal | none |
| `worker` | same Go module, `cmd/worker`, asynq | horizontal (the scheduler is singleton via asynq's leader-less PeriodicTaskManager plus idempotent jobs) | none |
| `migrate` | goose, one-shot | — | — |
| `postgres` | PostgreSQL 18 (official image) | 1 primary | named volume |
| `redis` | Redis 8 (official image), AOF on | 1 | named volume |
| `garage` | Garage v2.x (`dxflrs/garage`) | 1 node (MVP) | named volumes (meta, data) |
| `mailpit` | dev/staging only, catches SMTP | 1 | none |

## 2. Repository layout (monorepo)

```
.
├── AGENTS.md / CLAUDE.md        # agent operating manual (read first)
├── PRODUCT.md                    # product truth (impeccable)
├── DESIGN.md                     # visual system, written at the end of the first UI build (impeccable documenter)
├── package.json                  # root Bun workspace + task scripts (cross-platform)
├── scripts/                      # Bun TS scripts: dev orchestration, garage init, checks
├── api/                          # OpenAPI 3.1 contract (single source of truth)
│   └── openapi.yaml
├── apps/
│   ├── web/                      # Next.js (Bun)
│   │   ├── src/app/(marketing)/  # landing (Persuade surface, later)
│   │   ├── src/app/(auth)/       # /login /register /invite/[token]
│   │   ├── src/app/(app)/        # /today /pacts /pacts/[id] /pacts/[id]/days/[date] /review /settings
│   │   ├── src/components/       # ui/ (owned shadcn, restyled), passbook/, proof-editor/, ...
│   │   ├── src/lib/api/          # generated types (openapi-typescript) + openapi-fetch client
│   │   ├── src/i18n/             # next-intl config; messages/id.json, messages/en.json
│   │   └── tests/e2e/            # Playwright
│   └── server/                   # Go module github.com/andi-frame/lockedin/apps/server
│       ├── tools/                # separate module: goose, sqlc, oapi-codegen (run from that folder: `bun run go:tool <name>`)
│       ├── cmd/api/main.go
│       ├── cmd/worker/main.go
│       ├── cmd/tepatictl/main.go # admin CLI: seed, replay settlement, garage check
│       ├── internal/
│       │   ├── config/           # env parsing (caarlos0/env), validation
│       │   ├── domain/           # pure logic: pact terms, check-in FSM, ledger math, clock
│       │   ├── service/          # use-cases; transactions; calls domain + store + queue
│       │   ├── store/            # sqlc-generated queries + tx helpers (pgx)
│       │   ├── http/             # Fiber handlers (oapi-codegen strict server), middleware
│       │   ├── jobs/             # asynq task types, handlers, scheduler registration
│       │   ├── media/            # sniffing, ffmpeg/vips wrappers, limits
│       │   ├── storage/          # BlobStore interface: s3 (Garage) and fs drivers
│       │   ├── auth/             # argon2id, sessions, CSRF
│       │   └── notify/           # in-app + SMTP email
│       ├── db/migrations/        # goose SQL migrations
│       ├── db/queries/           # sqlc .sql files
│       └── sqlc.yaml
├── deploy/
│   ├── compose.yaml              # base: all services, profiles infra/app
│   ├── compose.dev.yaml          # dev overrides: source mounts + hot reload
│   ├── compose.prod.yaml         # staging/prod overrides: built images, edge, no source mounts
│   ├── docker/                   # Dockerfiles: web.Dockerfile, server.Dockerfile
│   ├── caddy/Caddyfile
│   ├── garage/garage.toml
│   └── env/                      # .env.example, .env.dev.example, .env.staging.example, .env.production.example
├── docs/                         # SPEC, ARCHITECTURE, RUNNING, PLAN, adr/, design/, diagrams/
└── .impeccable/                  # design workflow state (surface briefs, config)
```

**The contract flows one way:** `api/openapi.yaml` → `oapi-codegen` (Go strict-server interfaces + models) and `openapi-typescript` (web types). Change the YAML first, regenerate, then implement. CI fails if the generated code is stale.

## 3. Backend design (Go)

Layering, with dependencies pointing downward only:

```
http (Fiber handlers, DTO mapping)  ─┐
jobs (asynq handlers)               ─┼─▶ service (use-cases, tx boundaries) ─▶ domain (pure) 
cmd/tepatictl                       ─┘                 │
                                                       ├─▶ store (sqlc/pgx)
                                                       ├─▶ storage (BlobStore)
                                                       └─▶ queue (asynq client)
```

- `domain` has **no I/O**: terms validation, `NextStatus(checkIn, event, now) (Status, error)`, penalty clamp math, and deadline computation from `(terms, localDate)`. It is 100% unit-testable. The clock is an injected interface `Clock{ Now() time.Time }`.
- `service` owns transactions: `store.WithTx(ctx, func(q *store.Queries) error {...})`. Each transition (`SubmitProof`, `Approve`, `Reject`, `Dispute`, `ResolveDispute`, `Override`, `DeclareRest`, `SweepDeadlines`) is a single function used by both the API and the worker.
- After a transaction commits, tasks are enqueued (notifications and so on). If enqueue fails, it is logged and the periodic sweeps recover the state. **Never enqueue inside a transaction that might roll back.** For must-deliver side effects, use the `outbox` table: insert in the tx, then a worker relays it.
- Errors are domain error types mapped to RFC 9457 `application/problem+json` with stable `code` strings (`checkin.deadline_passed`, `pact.terms_mismatch`, …). The web app maps codes to i18n messages.
- IDs are UUIDv7 (time-ordered) generated in Go. Money is `int64` coins, never floats.
- Time: store `timestamptz` (UTC) and `date` for `local_date`. Convert only through `domain.Deadlines(terms, localDate)`.

### Middleware stack (Fiber v3, in order)

`requestid` → structured access log and metrics → `recover` → `cors` (dev only; prod is same-origin) → **rate limiter** (Redis storage; 300 req/min/IP global, stricter per route: auth 10/min, upload-intent 30/min) → session auth + CSRF (double-submit token header for unsafe methods; only register, login and the invite preview are public) → **idempotency** (`Idempotency-Key` header on mutating requests; the 2xx response is cached in Redis for 24 h, scoped by user, method and path; failures release the key) → body limit (1 MB JSON, enforced by Fiber while it reads the request; uploads never pass through the API) → handler.

The access log sits outside `recover` on purpose, so a panic is logged as the 500 it became. Logs carry the route pattern, never the raw path (invite tokens live in paths), and never bodies or tokens. `/healthz`, `/readyz` and `/metrics` are mounted at the origin root, are exempt from rate limits, and `/metrics` must not be proxied to the internet (Caddy, PLAN 7.2). Request IDs come from `X-Request-Id` and are echoed in every problem body.

### Load and burst handling

- The API is stateless, so scale with `--scale api=N`; Caddy load-balances (`lb_policy least_conn`).
- Use a `pgxpool` per instance (`max_conns = 4 × vCPU`). Add PgBouncer (transaction mode) only when `instances × max_conns > 80% of max_connections`.
- Heavy or slow work (media transcoding, emails, settlement) **never** runs in a request. It goes to asynq queues with priorities `critical: 6` (settlement), `default: 3` (notifications), `media: 1` (transcode), and a separate worker concurrency for `media` (2) to protect the CPU.
- Cache read-heavy aggregates in Redis (pot balance, the Today summary per user) with a 30 s TTL plus explicit invalidation on ledger or check-in writes. Postgres stays the truth.
- Use backpressure on media: if the `media` queue size exceeds `MEDIA_QUEUE_MAX` (default 500), the upload-intent endpoint returns `503` with `Retry-After`.

**Why asynq rather than raw Redis Streams, NATS, or RabbitMQ:** see ADR-0004. In short, it is Redis-backed (Redis is already required), Go-native, and has retries with backoff, scheduled and periodic tasks, unique tasks, priorities, and a web UI (asynqmon), with no extra broker to operate.

## 4. Data model (PostgreSQL)

The authoritative DDL lives in `apps/server/db/migrations`. This section is the design reference.

```sql
create table users (
  id uuid primary key, email citext unique not null, password_hash text not null,
  display_name text not null, locale text not null default 'id', timezone text not null default 'Asia/Jakarta',
  avatar_key text, email_verified_at timestamptz, created_at timestamptz not null default now()
);

create table pacts (
  id uuid primary key, title text not null, description text,
  status text not null check (status in ('draft','proposed','scheduled','active','settling','completed','cancelled')),
  created_by uuid not null references users(id),
  backer_id uuid not null references users(id),
  terms jsonb not null, terms_version int not null default 1, terms_hash text not null,
  timezone text not null, starts_on date not null, ends_on date not null,
  overrides_used int not null default 0,
  scheduled_at timestamptz, settled_at timestamptz, completed_at timestamptz,
  created_at timestamptz not null default now(), updated_at timestamptz not null default now()
);

create table pact_members (
  pact_id uuid references pacts(id) on delete cascade, user_id uuid references users(id),
  role text not null check (role in ('backer','doer')),
  line_color text not null,                       -- fixed per member (design: one line colour)
  accepted_terms_hash text, accepted_at timestamptz, signature_name text,
  rest_days_used int not null default 0,
  primary key (pact_id, user_id)
);

create table pact_invites (
  token_hash text primary key, pact_id uuid not null references pacts(id) on delete cascade,
  email citext, expires_at timestamptz not null, used_at timestamptz
);

create table check_ins (
  id uuid primary key, pact_id uuid not null references pacts(id), member_id uuid not null references users(id),
  reviewer_id uuid not null references users(id),
  local_date date not null,
  status text not null check (status in ('open','submitted','approved','auto_approved','rejected','disputed','missed','rest')),
  is_final boolean not null default false,
  cutoff_at timestamptz not null, submit_deadline timestamptz not null,
  submitted_at timestamptz, review_deadline timestamptz,
  decided_at timestamptz, dispute_deadline timestamptz, resolution_deadline timestamptz, override_deadline timestamptz,
  penalty_applied boolean not null default false,
  unique (pact_id, member_id, local_date)
);
create index check_ins_sweep_idx on check_ins (status, submit_deadline) where not is_final;
create index check_ins_review_idx on check_ins (reviewer_id, status) where status = 'submitted';

create table proofs (
  id uuid primary key, check_in_id uuid not null references check_ins(id), version int not null,
  body_doc jsonb not null, body_text text not null, word_count int not null,
  links jsonb not null default '[]', created_at timestamptz not null default now(),
  unique (check_in_id, version)
);

create table attachments (
  id uuid primary key, owner_id uuid not null references users(id), pact_id uuid not null references pacts(id),
  proof_id uuid references proofs(id),            -- null until attached on submit
  kind text not null check (kind in ('image','video','file')),
  status text not null check (status in ('awaiting_upload','uploaded','processing','ready','rejected')),
  staging_key text, media_key text, thumb_key text,
  declared_mime text, sniffed_mime text, declared_bytes bigint, stored_bytes bigint,
  width int, height int, duration_ms int, reject_reason text,
  created_at timestamptz not null default now(), ready_at timestamptz
);

create table decisions (                           -- audit of every review action (power is visible)
  id bigserial primary key, check_in_id uuid not null references check_ins(id),
  actor_id uuid references users(id),             -- null = system
  action text not null check (action in ('approve','reject','auto_approve','override','dispute','uphold','dismiss','dispute_expired','rest','missed')),
  reason text, created_at timestamptz not null default now()
);

create table ledger_entries (                      -- append-only; see SPEC §6
  id bigserial primary key, pact_id uuid not null references pacts(id),
  kind text not null check (kind in ('pot_initial','doer_miss','backer_miss','reversal','payout')),
  amount bigint not null, check_in_id uuid references check_ins(id), reverses_entry_id bigint references ledger_entries(id),
  idempotency_key text not null unique, note text, created_at timestamptz not null default now()
);
create index ledger_pact_idx on ledger_entries (pact_id, id);

create table payouts (
  pact_id uuid primary key references pacts(id), amount bigint not null,
  marked_paid_at timestamptz, marked_paid_note text, confirmed_at timestamptz
);

create table notifications (id bigserial primary key, user_id uuid not null, kind text not null, payload jsonb not null, read_at timestamptz, created_at timestamptz not null default now());
create table outbox (id bigserial primary key, topic text not null, payload jsonb not null, created_at timestamptz not null default now(), dispatched_at timestamptz);
```

Enforce append-only at the database level too:

```sql
create function forbid_mutation() returns trigger language plpgsql as $$ begin raise exception 'ledger_entries is append-only'; end $$;
create trigger ledger_no_update before update or delete on ledger_entries for each row execute function forbid_mutation();
```

## 5. HTTP API (summary; the full contract is `api/openapi.yaml`)

Base path `/api/v1`. Authentication uses session cookies. Errors use `problem+json`, with the status of every `code` listed under `ErrorCode` in the contract. `/healthz`, `/readyz`, and `/metrics` sit at the origin root. Go code is generated into `internal/http/api` (`bun run codegen`), and the web types into `apps/web/src/lib/api/schema.d.ts`.

| Method | Path | Purpose |
|---|---|---|
| POST | `/auth/register`, `/auth/login`, `/auth/logout` | session lifecycle |
| GET | `/me` | current user |
| GET | `/today` | aggregate for the Today screen: my open check-ins with deadlines, my review queue count, and a pot summary per active pact |
| GET/POST | `/pacts` | list mine / create draft |
| GET/PATCH | `/pacts/{id}` | detail / edit terms (draft/proposed only) |
| POST | `/pacts/{id}/propose` | send invite (returns invite link) |
| GET | `/invites/{token}` | preview terms for the invitee (public: the token is the credential) |
| POST | `/invites/{token}/join` | the invitee takes the doer slot; the terms hash changes, so they then call `accept` |
| POST | `/pacts/{id}/accept` | body `{terms_hash, signature_name}` |
| GET | `/pacts/{id}/ledger?cursor=` | passbook lines with running balance (window function) |
| GET | `/pacts/{id}/check-ins?from=&to=` | calendar |
| GET | `/check-ins/{id}` | detail: proofs (latest + history), attachments, decisions, dispute |
| PUT | `/check-ins/{id}/proof` | create or replace the proof draft and submit (`Idempotency-Key`) |
| POST | `/check-ins/{id}/rest` | declare a rest day |
| POST | `/check-ins/{id}/approve` · `/reject` · `/override` | reviewer actions (reason required for reject/override) |
| POST | `/check-ins/{id}/dispute` · `/dispute/resolve` | dispute flow |
| GET | `/review-queue` | submitted check-ins where I am the reviewer, sorted by `review_deadline` |
| POST | `/uploads` | upload intent → `{attachment_id, put_url, headers, expires_at}` |
| POST | `/uploads/{id}/complete` | client finished the PUT → enqueue processing |
| GET | `/attachments/{id}` | status and signed GET URLs (5 min) |
| POST | `/pacts/{id}/payout/mark-paid` · `/confirm` | settlement |
| GET | `/notifications` · POST `/notifications/read` | inbox |
| GET | `/healthz` · `/readyz` | liveness / readiness (DB + Redis + S3 ping) |

## 6. Upload and compression pipeline

```
client                         api                          garage                 worker (media queue)
  │ pick file                   │                              │                          │
  │ pre-check type/size,        │                              │                          │
  │ compress image in browser   │                              │                          │
  │ (browser-image-compression, │                              │                          │
  │  ≤ 2560px, webp q0.85)      │                              │                          │
  │── POST /uploads {kind,mime,bytes,pact_id} ─▶│              │                          │
  │                             │ validate limits, quota,      │                          │
  │                             │ rate limit, queue backpressure                          │
  │                             │ insert attachment(awaiting_upload)                      │
  │◀─ presigned PUT (staging/{id}), signed Content-Length + Content-Type, 10 min ─│       │
  │── PUT bytes ───────────────────────────────────────────────▶│                          │
  │── POST /uploads/{id}/complete ─▶│ HEAD object: size == declared? else reject           │
  │                             │ status=uploaded; enqueue media:process(id) ─────────────▶│
  │                             │                              │◀── GET staging object ───│
  │                             │                              │    sniff magic bytes (net/http.DetectContentType + h2non/filetype)
  │                             │                              │    image: vips → webp ≤2048 + thumb 480, strip metadata
  │                             │                              │    video: ffprobe (duration ≤180s) → ffmpeg 720p crf28 + poster
  │                             │                              │    file(pdf): validate header, keep
  │                             │                              │◀── PUT media/{pact}/{id}.* ─│
  │                             │                              │    delete staging object  │
  │                             │  status=ready|rejected (reason) ◀────────────────────────│
  │ poll GET /attachments/{id} (1s→5s backoff) until ready|rejected                        │
```

- **Defence in depth on size:** (1) a client pre-check, (2) the intent rejects declared bytes over the limit, (3) the signed `Content-Length` header means Garage rejects a mismatched PUT, (4) `complete` HEADs the object and rejects a mismatch, (5) the worker re-checks actual pixels and duration, (6) the post-transcode size is capped. Caddy also caps any request body to the API at 2 MB.
- **Garage presign caveat:** verify in integration test `TestPresignedPutRejectsWrongLength` that Garage enforces a signed `Content-Length`. If it does not, switch the `storage` driver option `UPLOAD_MODE=proxy`: the client PUTs to `/api/v1/uploads/{id}/body`, which streams to Garage through `io.LimitReader(max+1)`. Caddy then allows a larger body only on that route. Both modes must exist behind the same interface.
- Lifecycle: a periodic job deletes `awaiting_upload`/`uploaded` attachments older than 24 h and staging objects older than 24 h. Orphan `ready` attachments (never attached to a proof) are deleted after 7 days.
- Media is served via short-lived signed GET URLs (never public buckets). Only pact members can obtain them.
- The `fs` storage driver (for no-Docker mode) implements the same interface. It uses an HMAC-signed local URL `/api/v1/blob/{key}?sig=` and is not for production.

## 7. Frontend design (Next.js)

- **Runtime:** `bun --bun next dev|build|start`. Use Bun for package management (`bun.lock`).
- **Rendering:** route pages are Server Components that fetch from the Go API server-side (forwarding the session cookie) for first paint. Interactive islands use TanStack Query with the generated `openapi-fetch` client. Mutations send an `Idempotency-Key` (uuid generated per attempt) and the CSRF header.
- **Styling and UI:** Tailwind v4, shadcn/ui components **owned and restyled** to the direction contract (they never ship in their default look), Phosphor icons (one family, stroke weight fixed), Motion (`motion/react`) for the passbook-print signature move, honouring `prefers-reduced-motion`.
- **Rich text:** Tiptap 3 with StarterKit and Link, TaskList, Placeholder, CharacterCount, and a custom `attachmentImage` node that references `attachment_id` (never a raw URL). Pasted or dropped images are routed through the upload pipeline.
- **i18n:** next-intl, default `id`, and `en` available. No locale prefix in the URL. Locale comes from user settings or a cookie. All copy lives in `messages/*.json` and is never hard-coded.
- **PWA:** manifest plus a service worker for installability and a camera `capture` input. Offline support is not an MVP goal.
- **Formatting:** amounts use `Intl.NumberFormat('id-ID')`. Coins are shown as `1.000 koin` with the IDR equivalent `≈ Rp1.000.000`. Debit and credit always carry a sign and a D/K label as well as colour.
- **Design source of truth:** `.impeccable/surfaces/apps-web-src-app-app.md` (direction contract), then `DESIGN.md` once it is written. See `docs/design/README.md` for the workflow.

## 8. Auth and security

- Passwords use argon2id (`golang.org/x/crypto/argon2`, m=64MB, t=3, p=2). Sessions are opaque random 32-byte tokens in an `HttpOnly; Secure; SameSite=Lax` cookie. Redis stores the hash, the user ID, and a 30-day sliding TTL.
- CSRF uses a double-submit token for state-changing requests. All endpoints are same-origin behind Caddy.
- Authorisation checks run in the service layer, not only in handlers. Every pact-scoped query is filtered by membership.
- Upload hardening: MIME sniffing, ffmpeg/vips run with timeouts (`context.WithTimeout`, 120 s video and 20 s image) and resource limits, no shelling out with user-controlled strings (pass args as slices), and EXIF/GPS stripped.
- The rich-text JSON is validated against an allow-list (§8 of SPEC). The web renders it with Tiptap's static renderer and never with `dangerouslySetInnerHTML` of user HTML.
- Secrets come only from env and env files. Real env files are gitignored. Only `*.example` files are committed.
- Security headers come from Caddy: HSTS, a CSP (self plus the Garage media origin), `X-Content-Type-Options`, `Referrer-Policy`, and `Permissions-Policy: camera=(self)`.

## 9. Configuration (env)

All services read env vars. `deploy/env/.env.example` lists every variable with a comment. Key ones:

```
APP_ENV=dev|staging|production
APP_BASE_URL=http://localhost:3000
DATABASE_URL=postgres://tepati:tepati@localhost:5432/tepati?sslmode=disable
REDIS_URL=redis://localhost:6379/0
S3_ENDPOINT=http://localhost:3900   S3_REGION=garage   S3_ACCESS_KEY=…  S3_SECRET_KEY=…
S3_BUCKET_STAGING=tepati-staging     S3_BUCKET_MEDIA=tepati-media      S3_PUBLIC_ENDPOINT=http://localhost:3900
STORAGE_DRIVER=s3|fs                 FS_STORAGE_DIR=./.data/blobs       UPLOAD_MODE=presigned|proxy
UPLOAD_IMAGE_MAX_BYTES=15728640      UPLOAD_VIDEO_MAX_BYTES=209715200   UPLOAD_VIDEO_MAX_SECONDS=180
UPLOAD_FILE_MAX_BYTES=20971520       MEDIA_QUEUE_MAX=500
FFMPEG_PATH=ffmpeg                   VIPS_PATH=vips
SMTP_URL=smtp://localhost:1025       MAIL_FROM="Tepati <no-reply@tepati.local>"
SESSION_SECRET=…                     API_INTERNAL_URL=http://localhost:8080   # used by Next server-side
```

`S3_PUBLIC_ENDPOINT` differs from `S3_ENDPOINT` inside Docker. The API talks to `http://garage:3900`, but presigned URLs must be signed for the host the **browser** reaches. Sign with a client configured with the public endpoint.

## 10. Observability

- Logging uses `log/slog` JSON with `request_id`, `user_id`, and `pact_id`, and the web uses pino-style JSON. Never log proof bodies or tokens.
- `/metrics` (Prometheus) on the API and worker exposes request latency, asynq queue sizes, settlement transitions by type, and media processing time and failures.
- asynqmon UI (dev and staging only, behind basic auth in staging).

## 11. Testing strategy

| Layer | Tooling | What |
|---|---|---|
| domain | `go test` table tests | FSM transitions, deadlines across timezones, clamp math, terms validation |
| store/service | `go test -tags=integration` with `internal/testdb`: a throwaway, migrated database per test on the dev Postgres (`TEST_DATABASE_URL` or `DATABASE_URL`; CI uses a Postgres service container) | transactions, idempotency, concurrent sweeps (run 2 sweeps in parallel and assert no double penalty) |
| http | handler tests against the strict-server interface | auth, validation, problem codes |
| media | golden files (small fixtures in `testdata/`) | sniffing, rejection, output dimensions |
| web unit | `bun test` + Testing Library | formatters, reducers, editor schema |
| e2e | Playwright (`apps/web/tests/e2e`) against the full Docker stack with `CLOCK_OVERRIDE` enabled in dev | create → accept → submit → approve → miss → passbook shows lines |

A test-only endpoint `POST /api/v1/_test/clock` (compiled only with build tag `testclock`) lets e2e tests advance time so settlement runs deterministically.
