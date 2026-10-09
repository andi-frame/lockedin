# Handover: working in this repo as any agent

Written 2026-10-08, after the preparation of task 9.1, by the agent that built Phases 2 to 5 (Claude Code, Sonnet 5.5). It is for the **next agent, whatever it is** (Codex, Antigravity, another Claude model, a human). Nothing here depends on Claude-specific tools. `AGENTS.md` holds the rules, `docs/STATUS.md` holds the project state and decisions, and this file holds *how to actually get things done on this machine without repeating my mistakes*.

## 0. In one minute

- **Tepati** is a study-pact app (two peers, a coin IOU ledger, daily proof). Folder name is `lockedin`; product name is Tepati.
- **Done:** Phases 0 to 4 (the whole backend), **5.1** (the `apps/web` scaffold, PR #3), **5.2** (design tokens, dark desk-lamp theme, UI primitives, `/dev/kitchen-sink`, PR #4) **5.3** (auth guard `src/proxy.ts`, `/login`, `/register`, the signed-in shell, `auth.spec.ts`, PR #5) and **6.1** (the pact wizard, the agreement page and the invite flow, PR #6) **6.2** (the Today screen, PR #7) **6.3** (the proof editor and submission, PR #8) and **6.4** (the pact page with the coin book and calendar, PR #9) and **6.5** (the review queue and the check-in detail, PR #10) and **6.6** (the settlement and payout screens, PR #11) and **6.7** (the notifications inbox and settings, PR #12, which closes Phase 6) and **7.1** (the Dockerfiles, PR #13) and **7.2** (compose, Caddy, deploy scripts, backups, PR #14) and **7.3** (native mode polish, PR #15, which closes Phase 7) and **8.1** (CI, PR #16, green on GitHub) and **8.2** (the design finish review and `DESIGN.md`, PR #17) and **8.3** (the web-design-guidelines audit, PR #18) and **8.4** (the load test, PR #19) and **8.5** (the security review, PR #20, which closes Phase 8). The preparation of **9.1** (the runbook and the restore drill, with no server) is on branch `p9.1-staging`, committed, **not pushed or merged**; the deploy itself is not done.
- **Next task:** `docs/PLAN.md` **9.1 Deploy to a VPS** (prepared; it needs the owner's server, domain and SMTP) (9.2, the landing page, is done). Playbook in §9.
- **Phases 2 to 4 are merged into `main`** (pull request #1, https://github.com/andi-frame/lockedin/pull/1, merge commit `054e61a`, 2026-10-08, not squashed). `origin` is `https://github.com/andi-frame/lockedin.git`. The seven task branches (`p2.1-openapi` ... `p4.2-uploads`) were deleted on 2026-10-08 at the owner's request; their commits are in `main`. Do not delete branches unasked. The owner asked for the push and the merge explicitly each time: push, open PRs and merge only when they ask, and never force-push.
- **First three commands in a new Git Bash terminal:**

```bash
cd /c/ITB/Projects/lockedin
source scripts/dev-env.sh        # Go 1.26 toolchain + ffmpeg/libvips on PATH (see §3)
git status --short && git log --oneline -5
```

## 1. The owner and the rules they set

The owner is **andi-frame** (andifarhan1094@gmail.com). Student project, Windows machine.

| Rule | Why / how to apply |
|---|---|
| **Talk to the owner in casual Indonesian** ("bro", "gw/lu" is fine). Keep code, commits and technical docs in English. | Their own register. UI copy for the product is Indonesian too (`messages/id.json`, with `en.json` mirrored). |
| **No AI trail in git.** No `Co-Authored-By: ...` trailer, no "Generated with ..." line, in commits or PR bodies. Commit as the configured git user. | Explicit instruction on 2026-10-07 ("jangan ada trail claude"). Some harnesses inject an attribution reminder; it does not apply in this repo. |
| **One PLAN task per branch**, named `p<phase>.<task>-<slug>` (for example `p5.1-nextjs-app`). Do not start the next task in the same session unless the owner says so. | `AGENTS.md`. The owner decides pacing. |
| **Do not push, merge, force, or delete branches** without being asked. | Outward-facing and hard to undo. |
| **Ask, don't guess, on product behaviour.** If `docs/SPEC.md` does not answer it, ask, or pick the option that moves fewer coins and write the question down. | `AGENTS.md`. |
| **Settled decisions, do not re-ask:** name Tepati; coins are an IOU ledger (no real money); auto-approve plus dispute, the backer has limited, logged power; stack is pinned (Next.js on Bun, Go Fiber, Postgres, Redis, Garage); runs in Docker, hybrid and native modes; visual direction "Buku Tabungan" (passbook). | `docs/adr/`, `docs/SPEC.md`. |
| Docs are written so a **cheaper model can carry out the work**. Be explicit, give commands, name files. | The owner wants handovers like this one. |

## 2. Reading order and what each file is for

1. `AGENTS.md`: operating manual and the 10 invariants (ledger append-only, one implementation per transition, UTC plus injected clock, no floats in money, membership filter, and so on). Breaking an invariant is a bug even when tests pass.
2. `PRODUCT.md`: who it is for, the principles, the vocabulary.
3. `docs/STATUS.md`: **state, package map, every decision made so far, known gaps, and the brief for the next phase.** Update it at the end of each phase.
4. `docs/SPEC.md`: behavioural contract (state machines, deadlines, ledger rules, upload limits, notification table). Source of truth for logic. UI glossary in §1 (pact = *kontrak*, backer = *penyokong*, doer = *pelaku*, dispute = *sanggahan*, ...).
5. `docs/ARCHITECTURE.md`: services, layout, schema, API, upload pipeline (§6), frontend design (§7), security.
6. `docs/PLAN.md`: task list. Ticked tasks carry their commit range, the verification result and the deviations.
7. `docs/RUNNING.md`: run modes, ports, prerequisites, the Docker mount rules.
8. `docs/adr/`: why. ADR-0004 (queues), 0005 (storage, includes the Garage verification), 0010 (API conventions: errors, cursors, CSRF, idempotency) matter most for the web work.
9. UI work only: `.impeccable/surfaces/apps-web-src-app-app.md` (direction contract), `docs/design/README.md`.
10. `graphify-out/GRAPH_REPORT.md` is a generated code map. It was last refreshed after Phase 2, so it is **stale** for jobs, notify, media, storage and uploads. Trust `docs/STATUS.md` §4 over it.

## 3. The machine and the shell (read this before running anything)

Primary machine: Windows 11, Git Bash (also PowerShell). Docker Desktop, Bun, Go, Node are installed.

**Always start a terminal session with `source scripts/dev-env.sh`.** It exists because of two environment problems:

1. **Go version mismatch.** Installed Go is 1.25.3, `apps/server/go.mod` needs 1.26.0. Go auto-downloads 1.26 but then `go build/vet/test` fail with `compile: version "go1.26.0" does not match go tool version "go1.25.3"`. Every PLAN *Verify* line runs Go, and `bun run lint|test|codegen` shell out to Go. The script puts the cached 1.26 toolchain first on `PATH` and sets `GOTOOLCHAIN=local`. A permanent fix is installing Go 1.26 system-wide.
2. **ffmpeg, ffprobe, vips, vipsheader** were installed with `winget` on 2026-10-08 (`Gyan.FFmpeg`, `libvips.libvips`). winget only updates the PATH of terminals opened *afterwards*; older shells (and an already-running `dev:hybrid`) do not see them. The script adds their folders. The media code and its tests need all four.

If you start `bun run dev:hybrid`, do it from a shell where the script ran: `air` runs `go build`, and the worker calls ffmpeg/vips.

**Ports (dev):** web 3000, api 8080, Postgres **55432** (not 5432), Redis 6379, Garage S3 3900 (admin 3903), Mailpit UI 8025 (SMTP 1025), asynqmon 8081, worker metrics 9091. The root `.env` is gitignored and holds the Garage keys written by `bun run garage:init`.

**Seed data for looking at things:** `bun run db:seed` (active pact with overdue check-ins), `bun run db:seed -- invite` (proposed pact plus an invite email in Mailpit). Users `seed-backer@tepati.test` and `seed-doer@tepati.test`, password `tepati-seed-1234`.

## 4. Commands

```bash
bun run infra:up            # Postgres, Redis, Garage, Mailpit in Docker (infra:down / infra:reset / infra:status)
bun run db:migrate          # goose migrations on the dev DB (db:status, db:rollback, db:new)
bun run dev:hybrid          # infra + api + worker via air hot reload (no web yet). Ctrl+C stops it.
bun run codegen             # sqlc + oapi-codegen (Go strict server) + openapi-typescript. `-- --check` verifies freshness
bun run lint                # scripts tsc, OpenAPI lint, gofmt, go vet
bun run test                # bun script tests + Go unit tests (no DB)
cd apps/server && go test -race -p 3 -count=1 -tags=integration ./...   # everything, real Postgres/Redis/Garage
bun run ctl -- pact show <id>        # tepatictl: inspect a pact (check-ins, ledger, balance)
curl localhost:9091/metrics          # worker metrics
```

Before committing: `bun run lint && bun run test`; if you touched Go services, stores, storage or jobs also the integration run above; if you touched `api/openapi.yaml`, `db/queries/*.sql` or migrations, run `bun run codegen` and commit the generated output.

Verify a green integration run properly: read the output for `SKIP`. The media tests skip (with a message) if ffmpeg/libvips are not on PATH; the Garage tests skip without `S3_ACCESS_KEY`. Last full run (2026-10-08): all packages `ok` with `-race -p 3`; without `-p 3` the linker once died with `fatal error: runtime: cannot allocate memory`.

Integration tests create a throwaway database per test and use Redis logical DBs **auth 15, http 14, jobs 13**. A new package that needs Redis must take its own index (`testdb.RedisIn(t, n)`; next free is 12).

## 5. How work is done here (the loop)

1. `git checkout main && git pull --ff-only`, then `git checkout -b p<phase>.<task>-<slug>`. (Phases 2 to 4 were stacked on each other because nothing was merged until the end; start from `main` now, and stack only if a previous task is still unmerged.)
2. Read the task in `docs/PLAN.md` and the docs it points to. Use the contract first: API change = edit `api/openapi.yaml`, then `bun run codegen`; DB change = new goose migration, then sqlc queries, then codegen; behaviour change = update `docs/SPEC.md` in the same branch.
3. **Test first**: write the failing test, watch it fail for the right reason, then implement. For logic that must not silently regress, do a **mutation check**: break the code on purpose and confirm the test fails (done for the Garage length test and the EXIF-strip test; both are in PLAN/ADR). Money, time and state-machine code needs table-driven unit tests *and* an integration test against real Postgres.
4. Commit in small, reviewable steps: `feat(p4.2): ...`, `fix(p3.2): ...`, `docs(p4.2): ...`. Keep a PR under about 600 changed lines excluding generated code (`p2.3` broke this; it is split into three commits).
5. Run every command in the task's *Verify* line and read the output. If something fails or you skipped a step, say so plainly.
6. Close the task: tick it in `docs/PLAN.md` with the commit range, a **Result** line (what you ran and saw) and **Deviations** (what you did differently from the plan and why). Update `docs/STATUS.md` (state, decisions, gaps, next brief), `docs/ARCHITECTURE.md` / `RUNNING.md` / an ADR where behaviour changed. Then tell the owner what was done, what was verified, and what you decided on their behalf.

Definition of done is in `AGENTS.md`. UI changes additionally need Playwright screenshots at 390 and 1440 px in light and dark, and a look at all four.

## 6. Traps I fell into (so you do not)

**Shell and files (Windows):**
- **CRLF.** Editing files from Python with default `open()` or from `sed -i` can write CRLF and produce huge whole-file diffs. In Python use `open(p, encoding="utf-8", newline="")` for both read and write. After editing Go run `gofmt -w`; for other files `sed -i 's/\r$//' file`. Check `git diff --stat` for suspicious sizes.
- **Large heredocs in Git Bash** (a Bash command with a Go or Python program inside `<<'EOF'`) sometimes fail to parse (`unexpected EOF while looking for matching`), and backslashes inside Python-in-heredoc get re-interpreted. Write source files with your editor/file-write tool, and run longer Python as a script file.
- **MSYS path conversion.** Git Bash rewrites `/tmp/...` arguments but not ones containing `[` (so `vips thumbnail a.jpg "/tmp/o.webp[Q=80]"` fails). Use relative paths or Windows paths in manual CLI experiments. Go code (which uses Windows paths natively) is not affected.
- **`git add -A` picks up `graphify-out/2026-10-07/`**, a backup directory the owner deliberately left untracked. Add paths explicitly (`git add apps docs scripts deploy`) or `git reset graphify-out` before committing. I committed it by accident once and redid the commit.
- Killing dev processes on Windows: `air` and `tmp/*.exe` (`api.exe`, `worker.exe`) outlive a backgrounded `dev:hybrid`; stop them with PowerShell `Stop-Process` after you check what they are.

**Go and libraries:**
- **sqlc:** in a CTE `update ... returning id` an unqualified column is "ambiguous"; qualify it (`returning notifications.id`). Query changes need `bun run codegen`, and a changed signature breaks callers (for example `InsertNotification` now returns the id; `CreateAttachment` takes `created_at`).
- **Fiber v3:** `ctx.Context()` is a plain `context.Context`, so the authenticated user and client IP are stored with `c.SetContext` (read via `auth.UserFromContext`, `ClientIPFromContext`). `c.IP()` can be empty behind a trusted-proxy header: use `clientIP(c)`. One body limit for the whole app (so the fs blob route raises it and `jsonSizeCap` re-caps JSON). The generated strict server does not enforce schema `minimum` etc., so handlers validate what matters.
- **asynq:** `GetQueueInfo` on a queue that was never created returns an unwrapped internal error, not `ErrQueueNotFound` (list `Inspector.Queues()` first). There is no per-queue concurrency: media runs on a second server with its own pool. A task id makes enqueue idempotent (`ErrTaskIDConflict` / `ErrDuplicateTask` are success for us). Never enqueue inside a transaction; the outbox pattern exists for that.
- **Time:** never call `time.Now()` in code under test; use the injected `domain.Clock` (`FakeClock.Set/Advance` in tests). `local_date` is a pact-timezone date.
- **S3/Garage:** path-style addressing, presign against `S3_PUBLIC_ENDPOINT`, SDK default checksums switched off. Garage enforces signed `Content-Length`/`Content-Type`/expiry.
- Keys passed to storage must satisfy `storage.ValidKey` (build them from uuids only).

**Process:**
- context7 MCP was never authenticated in my sessions; I read module sources under `~/go/pkg/mod` instead. For Next.js 16, Tailwind v4, next-intl and Tiptap 3 use the official docs or an authenticated context7; do not trust memory for those versions.
- A passing test you never saw fail proves little. When I wrote a test and its implementation in the same step, I said so; do the red step.

## 7. Where each completed task lives

All merged into `main` through PR #1; the original task branches are kept on `origin`. Commit ranges are also in `docs/PLAN.md`.

| Task | Branch | Commits | What it delivers, and the proof |
|---|---|---|---|
| 2.1 OpenAPI contract | `p2.1-openapi` | `fd303f0`, `35b5d38` | `api/openapi.yaml` (31 operations), generated Go strict server and `schema.d.ts` |
| 2.2 Fiber app | `p2.2-fiber-middleware` | `3205c30`, `9f6b2c4` | problem+json, rate limits, idempotency, CSRF, health, metrics |
| 2.3 Handlers | `p2.3-handlers` | `b563a23`..`d03fe37` | all handlers, read models, keyset pages, payout settlement |
| (docs) | | `bb6098f`, `cdc7a94` | handoff status, ADR-0010, graph refresh |
| 3.1 Worker | `p3.1-worker-jobs` | `ab82852`..`3a9e6a1` | asynq server and scheduler, outbox relay, reminders, `tepatictl seed/pact show`, air files. Live: 6 overdue check-ins became `missed` in 35 s |
| 3.2 Email | `p3.2-notifications-email` | `ed2eca4`..`755c9e7` | invite mail event, `emailed_at` claims, `internal/notify` (SMTP, Indonesian copy), email tasks. Live: invite email in Mailpit |
| 4.1 BlobStore | `p4.1-blobstore` | `2ee6dce`..`b776f61` | `internal/storage` (s3 + fs), one contract suite for both. Garage enforces the signed length (mutation-checked) |
| 4.2 Uploads | `p4.2-uploads` | `ac7b781`..`f540af3` | `internal/media`, upload service and endpoints, `media:process` on its own server, `uploads:gc`. Golden tests plus an end-to-end test through Garage and the worker |
| 5.1 Web scaffold | `p5.1-nextjs-app` (merged, PR #3) | `21e40b4`..`b671f19` | `apps/web`: build, typecheck, eslint, 20 unit tests. Live: proxy to the API, CSRF 403 without and 204 with the header |
| 5.2 Tokens and primitives | `p5.2-tokens-primitives` (merged, PR #4) | `05047de`..`6008608` | OKLCH tokens in two themes with a WCAG test, owned primitives, `Amount`/`Countdown`/`StatusChip`/`MemberLine`, `/dev/kitchen-sink`. 154 web unit tests; screenshots at 390 and 1440, light and dark, looked at |
| 5.3 Auth and shell | `p5.3-auth-shell` (merged, PR #5) | `a9f3db3`..`884e87d` | `proxy.ts` guard with table-tested rules, login and register, rail and tab bar shell, Playwright `auth.spec.ts` (6 passed, 2 skipped on purpose). 243 web unit tests; screenshots at 390 and 1440, light and dark, looked at |
| 6.1 Pact wizard | `p6.1-pact-wizard` (merged, PR #6) | `632b9cd`..`2a94628` | `src/lib/pact` (dates, draft, terms summary, signing; 98 new unit tests), `Wizard`, `/pacts`, `/pacts/[id]`, `/invite/[token]`, `pact-create.spec.ts` (two contexts, ends `scheduled`). Screenshots at 390 and 1440, light and dark, looked at |
| 6.2 Today screen | `p6.2-today-screen` (merged, PR #7) | `01646bd`..`de149b3` | `AUTH_RATE_LIMIT_PER_MIN`, `tepatictl seed --scenario today` (integration-tested), `/today` (band, sections, rest day, review rows, mini passbook), `today.spec.ts` on both projects. 359 web unit tests; screenshots at 390 and 1440, light and dark, looked at |
| 6.3 Proof editor | `p6.3-proof-editor` | `d7467a7`..`344d270` | `src/lib/proof` (document analysis mirroring the server, evidence, files, tray; schema-vs-allow-list test), Tiptap 3 editor, attachment tray, `/pacts/[id]/days/[date]`, `proof.spec.ts` (250 MB refused in the browser, renamed file rejected by the server, real PNG and MP4 through the media worker). 439 web unit tests; screenshots at 390 and 1440, light and dark, looked at |
| 6.4 Pact page | `p6.4-pact-passbook` | `c1e0aa3..5aca5b4` | `src/lib/passbook.ts`, `src/lib/pact/calendar.ts`, `src/components/pact/{passbook,pact-calendar,pact-live,use-pact-live,rolling-amount}.tsx`, `(app)/pacts/[id]/running.tsx`, `tepatictl seed --scenario passbook` and `advance`, `Service.SweepCheckIns`, `passbook.spec.ts` (paging, calendar, a real miss printed, reduced motion). 459 web unit tests; screenshots at 390 and 1440, light and dark, and the line mid-print, looked at |
| 6.5 Review | `p6.5-review-queue` | `3cd9557..671a900` | `src/lib/checkin/{reason,timeline}.ts`, `src/components/checkin/*` (static-renderer proof view, gallery, decision timeline, decision actions, detail view), `/review`, `?of=` on the day page, `.proof-prose` styles, `tepatictl seed --scenario review`, `review.spec.ts` (approve; reject, dispute and uphold; override seen by both). 472 web unit tests; screenshots at 390 and 1440, light and dark, looked at |
| 6.6 Settlement | `p6.6-settlement` | `07ab78b..224a79b` | `src/lib/pact/settlement.ts`, `src/components/pact/{settlement-panel,payout-actions}.tsx`, `tepatictl seed --scenario settlement`, `settlement.spec.ts` (paid first then confirmed; doer confirms alone after a warning). 480 web unit tests; screenshots at 390 and 1440, light and dark, looked at |
| 6.7 Notifications | `p6.7-notifications` | `d2035d7..a55c373` | `src/lib/notification/inbox.ts`, `src/components/notification/notification-list.tsx`, `/notifications`, the bell as a link, Settings note, `notifications.spec.ts` (bell count, inbox copy and links, read state, doer's auto-approved row, Settings). 512 web unit tests; screenshots at 390 and 1440, light and dark, looked at |
| 7.1 Dockerfiles | `p7.1-dockerfiles` | `70baeca..745061b` | `deploy/docker/{server,web}.Dockerfile` (+ per-file `.dockerignore`), `scripts/deploy-build.ts`, `scripts/lib/images.ts` (+ test). Images run against the dev infra: api health and readiness green, worker clean start and stop, goose status, web `/login`. 989 MB and 339 MB |
| 7.2 Compose and Caddy | `p7.2-compose` | `7420358..7b1e17b` | `deploy/compose{,.dev,.prod}.yaml`, `deploy/caddy/Caddyfile`, `deploy/backup/`, `scripts/deploy-{up,scale}.ts` and `scripts/lib/deploy.ts` (+ test), `tepatictl probe` and `storage-init`, `RATE_LIMIT_PER_MIN`, forwarded client address. Dev and a local staging both ran the full e2e (37 passed, 5 skipped); scaling and backup pruning checked |
| 7.3 Native mode | `p7.3-native-polish` | `fe04c37` | `scripts/lib/checks.ts` (Postgres login, Redis version; tested), `scripts/lib/env.ts` (`layerNativeEnv`, `readRootEnv`, `pickDatabaseUrl`), `dev:native` migrates, `TEPATI_NATIVE=1` for db, ctl and the e2e seeds, `.env.native.example`, `E2E_TIMEOUT_MS`. Full e2e passed natively with a throwaway Postgres 18, Redis 8 in WSL and the fs driver |
| 8.1 CI | `p8.1-ci` | `4245bf0..369d5ea` | `.github/workflows/ci.yml` (check, integration, e2e on the built images), `scripts/deploy-env-local.ts` and `scripts/lib/localenv.ts` (+ test), `retries: 1` on CI. Green on GitHub (PR #16) after two fixes: a one-frame-video poster defect in `internal/media` and a race in `proof.spec.ts` |
| 8.2 Design finish | `p8.2-impeccable-finish` | `cfa8ab2..f5d46d7` | an inline finish review (44 captures; two fixes: links underlined at rest, guilloche off the rail), `DESIGN.md`, `.impeccable/design.json` |
| 8.3 Guidelines audit | `p8.3-web-guidelines` | `5fa5082` | `src/lib/unsaved.ts` (+ test), `src/lib/use-focus-on-desktop.ts`, autocomplete, ellipsis, theme-color, touch and overscroll fixes; accepted exceptions listed in PLAN 8.3 |
| 8.4 Load test | `p8.4-load-test` | `b47ec02` | `tests/load/today.js`, `scripts/load.ts`, `scripts/lib/load.ts` (+ test). 200 reads and 20 writes a second for 5 minutes: read p95 12.9 ms, write p95 23.6 ms, no errors, on a laptop (not a 2 vCPU verdict) |
| 8.5 Security review | `p8.5-security-review` | `b82771c` | a structural test that pins every scoped operation to an authorization test, `--pull` on image builds, x/net bump, CI `vulncheck`, a proof-link test; the checklist and its evidence are in PLAN 8.5 |

Phases 0 and 1 (repository foundation, schema, domain rules, services) are on `main`.

## 8. Using this repo with a non-Claude agent

`AGENTS.md` names several Claude Code skills. They are optional helpers, and the repo does not depend on them.

| `AGENTS.md` says | If you do not have it |
|---|---|
| superpowers (TDD, planning, review, verification) | Follow §5 here: red, green, refactor; verify before claiming done; review your own branch diff against `AGENTS.md` invariants. |
| impeccable, taste-skill, web-design-guidelines (UI) | The skill files are plain Markdown on this machine: `~/.claude/skills/impeccable/SKILL.md` and `reference/craft-floor.md`, `~/.claude/skills/taste-skill/SKILL.md`, `~/.claude/skills/web-design-guidelines/`. Read them as checklists. The authoritative design rules are in the repo: `.impeccable/surfaces/apps-web-src-app-app.md`, `docs/design/README.md`, `docs/adr/0009-visual-direction.md`. Do UI **code-first** (`.impeccable/config.json` says `"buildPath": "code"`); do not generate comp images. |
| context7 | Official docs for the exact versions in `package.json` / `go.mod`. |
| playwright | `bunx playwright` / `@playwright/test` directly for screenshots (390 and 1440 px, light and dark) and the e2e tests. |
| graphify, archify | Optional. `graphify-out/GRAPH_REPORT.md` is stale (§2). `docs/diagrams/` is produced by archify; regenerate only if the architecture changes. |

If your tool wants its own instruction file (`GEMINI.md`, `.codex/...`), make it a short pointer to `AGENTS.md` and this file. Do not copy rules into several places; they drift.

## 9. Playbook for the next task: 9.1 on a real server

**9.1 is prepared, not done.** `docs/RUNBOOK.md` is the script: read it first. What is verified: the first deploy, update, scaling and backup on a local staging stack (`DOMAIN=localhost`), and the restore drill, including the way back. What is not: anything on a real server (DNS, a Let's Encrypt certificate, real SMTP, the drill on the server, off-host copies). Ask the owner for: the host (or access to it), the domain and DNS control, the SMTP provider and sender, and where to copy the dumps. If the owner would rather run the commands themselves, give them the runbook sections 2, 3 and 6 in order and read the output they paste; `deploy:up` prints what is wrong with an env file before it does anything.

1. `git checkout main && git pull --ff-only && git checkout -b p9.1-deploy` (the preparation is on `p9.1-staging` until it is merged). CI runs on every pull request; keep it green.
2. After the real deploy: tick 9.1 in PLAN with the date, the host class and the drill's result, and write the drill's date in RUNBOOK section 6. Anything the real server taught you goes into the runbook's troubleshooting table.

**9.2 the landing page is done** (`/`, see PLAN 9.2 and the brief in `.impeccable/surfaces/apps-web-src-app-page-tsx.md`). Screenshots of a UI change: the Playwright MCP may fail to connect; a throwaway Bun script using `chromium` from `@playwright/test` in `apps/web` works (delete it after).

The owner decided on 2026-10-09 that the MVP gets `PATCH /me` and per-kind email preferences (PLAN 9.4, not built). The `starts_on` rule is open: the owner said "later".

## 10. Questions that are the owner's to answer (do not decide silently)

- Whether later PRs should be merged with merge commits (as PR #1 was, to keep the hashes in `docs/PLAN.md` valid) or squashed.
- Whether to install Go 1.26 system-wide (removes the `dev-env.sh` Go workaround).
- A real SMTP provider, a domain and DNS for staging and production (both `<DOMAIN>` and `media.<DOMAIN>` must resolve to the host; Caddy gets the certificates by itself). Staging can keep Mailpit.
- HEIC/AVIF handling: the sniffer accepts them, but decoding depends on the libvips build and was not tested with a real HEIC file.
- (Decided 2026-10-09: the MVP gets `PATCH /me` and per-kind email preferences, PLAN 9.4. Settings stays read-only until it is built.)
- A "resend invite" endpoint (the invite email is the only place the plaintext token is mailed; see `docs/STATUS.md §6`).

## 11. Before you stop a session

- [ ] Everything you intended is committed, with no stray files (`git status --short` shows only `graphify-out/2026-10-07/`).
- [ ] `docs/PLAN.md` ticked for finished tasks (range, Result, Deviations); unfinished work described, not ticked.
- [ ] `docs/STATUS.md` updated: state table, branch line, new decisions, new gaps, the brief for the next task. Fix stale sentences instead of piling on new ones.
- [ ] This file still tells the truth (branch list in §0 and §7, next task in §9).
- [ ] You told the owner, in Indonesian, what was done, what was verified, what you decided for them, and what you did **not** do.
