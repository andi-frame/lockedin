# ADR-0008: Three run modes driven by Bun scripts

Status: Accepted (2026-10-07)

## Context
The owner needs three ways to run the system:
- Everything in Docker, for dev, staging, and prod.
- No Docker at all.
- Hybrid: infrastructure in Docker, apps running natively.

The main dev machine runs Windows, which often has no `make`.

## Decision
- Root `package.json` scripts call TypeScript scripts in `scripts/`, run by Bun. The web app needs Bun anyway, and it works across platforms.
- Compose uses one base file with profiles (`infra`, `app`, `edge`, `tools`, `backup`), plus `compose.dev.yaml` and `compose.prod.yaml` overrides. Staging and production differ only by their env file.
- Native mode swaps Garage for the `fs` storage driver. Postgres and Redis must be installed locally. On Windows, get Redis through Memurai or WSL.

## Consequences
- One orchestrator script, `scripts/dev.ts`, handles process spawning, log prefixing, and shutdown, so every mode behaves the same way.
- The mount rules in `docs/RUNNING.md §4` are mandatory.
