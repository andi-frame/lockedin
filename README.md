# Tepati

> *Tepati janji belajarmu.* A pact between two friends: one puts up a coin pot, the other proves every day that they studied. Each missed day takes coins out of the pot.

**Status:** the backend is complete (Phases 0 to 4: API, worker, email, uploads and media). The web app is scaffolded (task 5.1: Next.js 16 on Bun with the typed API client); the design tokens, the dark theme and the UI primitives are in (task 5.2, `/dev/kitchen-sink` in dev); the auth pages, app shell and screens (5.3 onwards) are next. Read [`docs/HANDOVER.md`](docs/HANDOVER.md) and [`docs/STATUS.md`](docs/STATUS.md) first; the task list is [`docs/PLAN.md`](docs/PLAN.md).

## How it works

1. **A (backer)** creates a pact. Example: 30 days, pot of 1000 coins (≈ Rp1.000.000), 50 coins lost per missed day. A can also commit, in which case each day A misses adds 50 coins to the pot.
2. **B** reviews the terms, signs them, and the pact starts.
3. Every day before the cutoff, each doer submits **proof**: rich text, photos, video, links, or a PDF.
4. The other person reviews it. Silence means auto-approve after 24 h. A rejection needs a reason and can be disputed. The backer has a limited number of overrides, and every one is visible to B.
5. Missed days are settled automatically into an append-only **buku koin** (passbook).
6. At the end, A pays B the remaining pot outside the app, and both confirm. In the MVP, no money moves through Tepati.

## Stack

Next.js 16 on Bun · Go 1.26 + Fiber v3 · PostgreSQL 18 · Redis 8 + asynq · Garage (S3) · Caddy · Docker Compose (dev / staging / prod) plus hybrid and native modes.

## Quick start

```bash
bun install
bun run setup        # env files + secrets
bun run dev:hybrid   # Postgres/Redis/Garage/Mailpit in Docker, apps native
```

Other modes: `bun run dev:docker` runs everything in Docker, and `bun run dev:native` runs with no Docker at all. See [`docs/RUNNING.md`](docs/RUNNING.md).

## Documentation map

| File | For |
|---|---|
| [`AGENTS.md`](AGENTS.md) | Rules and workflow for coding agents (and humans) |
| [`docs/HANDOVER.md`](docs/HANDOVER.md) | How to work here as any agent: environment, owner's rules, work loop, traps, next-task playbook |
| [`docs/STATUS.md`](docs/STATUS.md) | Where the project stands, decisions made, known gaps, brief for the next phase |
| [`PRODUCT.md`](PRODUCT.md) | Users, purpose, principles, terminology |
| [`docs/SPEC.md`](docs/SPEC.md) | Behaviour: pact lifecycle, check-in state machine, ledger, settlement |
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | Services, layout, data model, API, uploads, security |
| [`docs/RUNNING.md`](docs/RUNNING.md) | Run modes, commands, Docker mount rules, deploy |
| [`docs/PLAN.md`](docs/PLAN.md) | Phased task list with acceptance criteria |
| [`docs/adr/`](docs/adr/) | Architecture decision records |
| [`docs/design/`](docs/design/) | Design workflow, visual direction, references |
| [`docs/diagrams/`](docs/diagrams/) | Interactive architecture, flow, and state diagrams (open in a browser) |
