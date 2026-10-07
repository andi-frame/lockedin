# Handover: working in this repo as any agent

Written 2026-10-08, after task 5.1, by the agent that built Phases 2 to 5.1 (Claude Code, Sonnet 5.5). It is for the **next agent, whatever it is** (Codex, Antigravity, another Claude model, a human). Nothing here depends on Claude-specific tools. `AGENTS.md` holds the rules, `docs/STATUS.md` holds the project state and decisions, and this file holds *how to actually get things done on this machine without repeating my mistakes*.

## 0. In one minute

- **Tepati** is a study-pact app (two peers, a coin IOU ledger, daily proof). Folder name is `lockedin`; product name is Tepati.
- **Done:** Phases 0 to 4 (the whole backend) and **5.1** (the `apps/web` scaffold: Next 16, Tailwind v4, next-intl, TanStack Query, the typed API client with CSRF and idempotency helpers). 5.1 is on branch `p5.1-nextjs-app`, committed, **not pushed or merged**.
- **Next task:** `docs/PLAN.md` **5.2 Design tokens and primitives**. Branch `p5.2-tokens-primitives`, cut from `p5.1-nextjs-app` while that is unmerged (otherwise from `main`). Playbook in §9.
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
| 5.1 Web scaffold | `p5.1-nextjs-app` | `21e40b4`..`b671f19` | `apps/web`: build, typecheck, eslint, 20 unit tests. Live: proxy to the API, CSRF 403 without and 204 with the header |

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

## 9. Playbook for the next task: 5.2 Design tokens and primitives

1. `git checkout p5.1-nextjs-app && git checkout -b p5.2-tokens-primitives` (or from `main` once 5.1 is merged). `source scripts/dev-env.sh`, `bun install`.
2. This is UI work. Load the *impeccable* skill (or read `~/.claude/skills/impeccable/SKILL.md`; the extend-an-existing-surface path applies, no new direction round), then read `~/.claude/skills/impeccable/reference/craft-floor.md` right before editing. Read the whole direction contract `.impeccable/surfaces/apps-web-src-app-app.md`, `docs/design/README.md`, `docs/adr/0009-visual-direction.md`, and `AGENTS.md` "UI work". Read Next 16 and Tailwind v4 docs for what you touch (`apps/web/node_modules/next/dist/docs/`, tailwindcss.com); context7 is unauthenticated.
3. Translate the contract's hex values into OKLCH tokens in `apps/web/src/styles/tokens.css` (import it from `globals.css`, expose it to Tailwind with `@theme`), a light theme and a designed dark "desk lamp" theme (not an inverted one). Add the owned shadcn-style primitives (button, input, textarea, dialog, sheet, tabs, select, toast, tooltip, badge) restyled to the contract, plus `Amount`, `Countdown`, `StatusChip`, `MemberLine`. Phosphor icons only.
4. Done when `/dev/kitchen-sink` (dev builds only) shows every primitive in both themes at 390 and 1440 px. Take Playwright screenshots of all four combinations and look at them, then run `impeccable detect --json <changed files>` once.
5. Tests: `Amount` and `Countdown` formatting are logic, so write them table-driven first (`Intl.NumberFormat('id-ID')`, no floats in money paths). All copy goes in `messages/id.json` with the `en.json` mirror.
6. Verify per PLAN (`bun run build`, screenshots), then `bun run lint && bun run test`, tick 5.2 in PLAN with range, Result and Deviations, update `docs/STATUS.md`, and stop. 5.3 (auth pages, shell, `auth.spec.ts`) is a separate branch. It needs a way to move time for e2e; there is no test-clock endpoint yet (`CLOCK_OVERRIDE` exists in config only).

Things the web app must provide because the backend already points at them: routes `/pacts/<id>`, `/review`, `/invite/<token>` (emails link there), and the upload client flow described in `docs/STATUS.md §7`.

## 10. Questions that are the owner's to answer (do not decide silently)

- Whether later PRs should be merged with merge commits (as PR #1 was, to keep the hashes in `docs/PLAN.md` valid) or squashed.
- Whether to install Go 1.26 system-wide (removes the `dev-env.sh` Go workaround).
- A real SMTP provider and TLS settings for staging and production.
- HEIC/AVIF handling: the sniffer accepts them, but decoding depends on the libvips build and was not tested with a real HEIC file.
- A "resend invite" endpoint (the invite email is the only place the plaintext token is mailed; see `docs/STATUS.md §6`).
- A test-clock mechanism for e2e tests (Phase 5.3 and 6.x).

## 11. Before you stop a session

- [ ] Everything you intended is committed, with no stray files (`git status --short` shows only `graphify-out/2026-10-07/`).
- [ ] `docs/PLAN.md` ticked for finished tasks (range, Result, Deviations); unfinished work described, not ticked.
- [ ] `docs/STATUS.md` updated: state table, branch line, new decisions, new gaps, the brief for the next task. Fix stale sentences instead of piling on new ones.
- [ ] This file still tells the truth (branch list in §0 and §7, next task in §9).
- [ ] You told the owner, in Indonesian, what was done, what was verified, what you decided for them, and what you did **not** do.
