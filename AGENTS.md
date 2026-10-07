# AGENTS.md: operating manual for coding agents

This file applies to every agent working in this repo: Claude Opus, Sonnet, or Haiku, Codex, and others. Humans should read it too. `CLAUDE.md` imports it.

## What this project is

**Tepati** is a web app for keeping study promises. Two peers sign a *pact*: the *backer* funds a coin *pot*, and the *doer* submits daily *proof* (rich text, photos, video, links) before a cutoff. Each missed day takes coins out of the pot. If the backer also commits and misses a day, coins are added to the pot. Coins are an IOU ledger, and no real money moves through the app. The folder is still called `lockedin`; the product name is Tepati.

## Read in this order

1. `PRODUCT.md`: who it's for, the principles, and the terminology.
2. `docs/STATUS.md`: where the project stands, what exists, decisions made so far, known gaps, and the brief for the next phase. Start here when resuming work.
3. `docs/SPEC.md`: the behavioural contract (state machines, deadlines, ledger rules). It is the source of truth for logic.
4. `docs/ARCHITECTURE.md`: services, repo layout, data model, API, upload pipeline, security.
5. `docs/PLAN.md`: the task list. Pick the first unchecked task you are allowed to work on.
6. `docs/RUNNING.md`: how to run, test, and deploy, including the **Docker mount rules**.
7. `docs/adr/`: why things are the way they are. Read the ADR for an area before changing it.
8. For any UI work: `.impeccable/surfaces/apps-web-src-app-app.md` (the direction contract), `docs/design/README.md`, and `DESIGN.md` once it exists.

If `graphify-out/GRAPH_REPORT.md` exists, read it before searching the code. It maps the codebase and saves exploration time. Refresh it with `/graphify . --update` after large changes.

## How to work

- **One PLAN task per branch and PR.** Branch name: `p<phase>.<task>-<slug>`, for example `p1.3-domain-fsm`. Keep PRs reviewable, under about 600 changed lines excluding generated code.
- **Start from the contract.** API change: edit `api/openapi.yaml` first, then `bun run codegen`. DB change: write a new goose migration, then the sqlc queries, then `bun run codegen`. Logic change: update `docs/SPEC.md` in the same PR if behaviour changes.
- **Test first for logic.** Write the failing test, make it pass, then refactor. Money, time, and state-machine code (tasks marked 🔒 in PLAN) needs table-driven unit tests **and** an integration test against real Postgres.
- **Verify before claiming done.** Run every command in the task's *Verify* line and read the output. If something fails or you skipped a step, say so plainly in the PR description. A task is done only when its *Done when* criteria are demonstrably met.
- **Check library APIs against current docs.** Use the context7 MCP tools (`resolve-library-id`, then `get-library-docs`) for Fiber v3, asynq, pgx, sqlc, goose, oapi-codegen, Next.js 16, Tailwind v4, Tiptap 3, next-intl, TanStack Query, Motion, and Garage. Training data for several of these predates their current major versions.
- **Ask, don't guess, on product decisions.** If SPEC doesn't answer a behavioural question, stop and ask the user, or write the question in the PR and pick the most conservative behaviour, the one that moves fewer coins.

## Invariants (breaking one is a bug, even when tests pass)

1. The ledger is append-only, and the pot balance is `SUM(ledger_entries.amount)`. Never add a mutable balance column. Never update or delete ledger rows.
2. Every coin movement has a unique `idempotency_key` and happens in the same DB transaction as the status change that caused it.
3. Each state transition has exactly one implementation, in `internal/service`, used by both the API and the worker. Pure rules live in `internal/domain`, which has no I/O.
4. Time: store UTC `timestamptz`, and store `local_date` as a `date` in the pact timezone. Compute deadlines only through `domain` helpers. Code under test never calls `time.Now()` directly; it uses the injected `Clock`.
5. Money is `int64` coins. No floats anywhere in money paths, including the frontend (format with `Intl.NumberFormat`).
6. Nobody reviews their own check-in. Backer power (override, dispute resolution) applies only to check-ins the backer reviews, and every use writes a `decisions` row that both members can see.
7. Uploads never pass through the API in presigned mode. File type is sniffed server-side and never trusted from the client. EXIF/GPS is stripped. Limits are enforced on the server.
8. Every pact-scoped query is filtered by membership in the service layer. A non-member gets 404.
9. Docker: data lives only in named volumes. Never bind-mount host `node_modules` or `.next`. Production images have no source mounts. See `docs/RUNNING.md §4`.
10. No secrets in git. Only `*.example` env files are committed.

## Code conventions

**Go** (`apps/server`)
- Use the standard layout described in ARCHITECTURE §2–3.
- Errors are wrapped with `%w`. Domain errors map to problem+json `code`s.
- `gofmt` and `go vet` (run by `bun run lint`).
- Name things in the SPEC's vocabulary: `Pact`, `CheckIn`, `Proof`, `LedgerEntry`, `Backer`, `Doer`.

**TypeScript** (`apps/web`)
- Strict mode, no `any`. API types come only from the generated `schema.d.ts`.
- Server Components by default. `"use client"` only for interactive leaves.
- All user-facing copy lives in `messages/id.json` (Indonesian first, natural and peer-to-peer in tone). Mirror every key in `en.json`.

**SQL**
- snake_case. Every foreign key is indexed.
- Sweep queries use `FOR UPDATE SKIP LOCKED LIMIT $n`.

**Comments:** explain *why*, not *what*. Link SPEC sections for rule-heavy code, for example `// SPEC §5: review deadline counts from cutoff`.

## UI work

UI is an Operate surface with a committed visual world: Buku Tabungan, the bank passbook. Steps:

1. Load the *impeccable* skill. It runs `impeccable context` and loads PRODUCT.md plus the surface brief. Follow its routing. For new screens inside the established shell, use its "extend an existing surface" path; no new direction round is needed.
2. Read `~/.claude/skills/impeccable/reference/craft-floor.md` right before editing UI.
3. Use the *taste-skill* sections 3, 4.4–4.6, 6, and 9 as a pre-flight checklist (states, contrast, forms, shape lock, reduced motion, banned AI tells). Its landing-page and hero rules don't apply to the app shell.
4. Respect the contract: cover teal for the shell, stamp violet **only** for human decisions, highlighter yellow **only** for today, a fixed colour per member, Geist / Geist Mono with tabular figures for numbers, radius 6/10, Phosphor icons only, no streak flames, no confetti.
5. After building, take Playwright screenshots at 390 and 1440 px, in light and dark, and look at them. Run `impeccable detect --json <changed files>` once.
6. Before release, run *web-design-guidelines* over the changed UI.

## Skills and tools available in this environment

| Need | Use |
|---|---|
| Plan a multi-step change, brainstorm, write plans | *superpowers:brainstorming*, *superpowers:writing-plans*, *superpowers:executing-plans* |
| Test-first development | *superpowers:test-driven-development* (or *tdd*) |
| Debugging a failure | *superpowers:systematic-debugging* |
| Before saying "done" | *superpowers:verification-before-completion* |
| Parallel independent tasks (⇄ in PLAN) | *superpowers:dispatching-parallel-agents* / *superpowers:subagent-driven-development* |
| Code review of your branch | *superpowers:requesting-code-review* or `/code-review` |
| Current library docs | context7 MCP |
| Browser checks, screenshots, e2e | playwright MCP / Playwright test runner |
| UI design, critique, polish, audit | *impeccable* (subcommands: `critique`, `audit`, `polish`, `harden`, `clarify`, …) |
| Anti-slop checklist | *taste-skill* |
| UI guideline compliance | *web-design-guidelines* |
| Architecture or flow diagrams | *archify*. Outputs go to `docs/diagrams/`. Regenerate when the architecture changes. |
| Codebase map | *graphify* (`/graphify .`, `/graphify query "…"`) |
| Finding more skills | *find-skills*, but only install well-known, reviewed sources |

## Notes for smaller or faster models (Sonnet, Haiku)

- Do exactly one PLAN task. Don't start the next one in the same session unless the user asks.
- Re-read the task's *Read* sections before coding; most mistakes come from skipping SPEC §5–§7.
- Prefer the simplest implementation that satisfies *Done when*. Don't add features listed in SPEC §11 (out of scope).
- For UI, use the impeccable **code-first** path (`.impeccable/config.json` has `"buildPath": "code"`). Don't generate comp images.
- If a Verify command fails twice for the same reason, stop and report what you tried instead of looping.

## Definition of done (every PR)

- [ ] The PLAN task's *Done when* is met, and its *Verify* commands passed (paste the summarised output in the PR).
- [ ] `bun run lint` and `bun run test` pass. `bun run test:integration` passes if Go service, store, or storage code changed.
- [ ] Docs are updated where behaviour, architecture, or run commands changed (SPEC / ARCHITECTURE / RUNNING / ADR).
- [ ] UI changes: screenshots at 390 and 1440 (light and dark) attached, plus the impeccable detector run.
- [ ] No secrets, no debug leftovers, no unrelated refactors.
