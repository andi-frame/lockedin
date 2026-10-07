# Running Tepati: dev, staging, production

There are three run modes, and every one is driven by **Bun scripts in the root `package.json`**. Bun is cross-platform, so the same commands work on Windows (PowerShell or Git Bash), macOS, and Linux without `make`.

| Mode | Infra (Postgres, Redis, Garage, Mailpit) | Apps (web, api, worker) | Command |
|---|---|---|---|
| **docker** | Docker | Docker (hot reload via source mounts) | `bun run dev:docker` |
| **hybrid** (recommended for daily dev) | Docker | native processes on the host | `bun run dev:hybrid` |
| **native** | installed on the host, no Docker | native | `bun run dev:native` |
| staging / production | Docker | Docker (built images, no source mounts) | `bun run deploy:up -- --env staging\|production` |

## 1. Prerequisites

| Tool | Version | Needed for |
|---|---|---|
| Bun | ≥ 1.3.14 (`bun upgrade`) | all modes |
| Docker Desktop / Engine + Compose v2 | ≥ 27 | docker, hybrid, deploy |
| Go | ≥ 1.26 (an older `go` auto-downloads 1.26 via `GOTOOLCHAIN=auto`) | hybrid, native |
| ffmpeg + libvips CLI (`vips`) | ffmpeg ≥ 6, vips ≥ 8.15 | hybrid and native worker (media). On Windows: `winget install Gyan.FFmpeg` and `winget install libvips.libvips` (then `source scripts/dev-env.sh` in a shell opened before the install, see `docs/HANDOVER.md §3`). winget adds their `bin` folders to the *user* PATH, which only new terminals see. Both also provide `ffprobe` and `vipsheader`, which the worker and the media tests call. |
| air (`go install github.com/air-verse/air@latest`) | latest | Go hot reload in hybrid/native |
| goose, sqlc, oapi-codegen | pinned in `apps/server/tools.go` and run via `go run` | codegen and migrations |
| PostgreSQL 18, Redis 8 | native mode only | Windows: Postgres installer, and Redis via **Memurai** or WSL. Garage has no Windows build, so native mode uses `STORAGE_DRIVER=fs`. |

## 2. First-time setup

```bash
bun install                       # installs the workspace (web + scripts)
bun run setup                     # copies deploy/env/*.example → .env files if missing, generates secrets, runs codegen
```

`bun run setup` is idempotent and never overwrites an existing `.env*`.

## 3. Commands (root `package.json` → `scripts/*.ts`)

| Command | What it does |
|---|---|
| `bun run infra:up` | `docker compose -f deploy/compose.yaml --env-file .env --profile infra up -d --wait`, then runs `garage:init` |
| `bun run infra:status` | `docker compose ps` for the infra profile |
| `bun run infra:down` | stops infra, **keeping volumes** |
| `bun run infra:reset` | stops infra and **deletes volumes** (asks for confirmation; `--yes` to skip) |
| `bun run garage:init` | idempotent: assigns the layout, creates key `tepati-app` and buckets `tepati-staging`/`tepati-media`, grants permissions, sets CORS on the staging bucket. The S3 key pair is generated once into `.env` (and `deploy/env/.env.dev`) and *imported*, so it survives `infra:reset` |
| `bun run s3:smoke` | lists both buckets and does a put/get/delete round-trip with the app key |
| `bun run db:migrate` / `db:rollback` / `db:new <name>` | goose against `DATABASE_URL` |
| `bun run db:seed [-- overdue\|invite]` | `tepatictl seed --scenario …`. `overdue` (default): an active pact that started 3 days ago with check-ins past their deadline, which a running worker marks `missed` within a minute or two. `invite`: a proposed pact with an open invite link; with the worker running, the invite email to `seed-invitee@tepati.test` shows up in Mailpit (`http://localhost:8025`) within seconds. Both reuse the users `seed-backer@tepati.test` and `seed-doer@tepati.test` (password `tepati-seed-1234`) and print the pact id |
| `bun run ctl -- <args>` | `tepatictl` with `.env` loaded, for example `bun run ctl -- pact show <id>` (members, every check-in, the ledger, the balance) |
| `bun run codegen` | sqlc, then OpenAPI → Go strict server (`internal/http/api`) and web types (`apps/web/src/lib/api/schema.d.ts`). `-- --check` fails if regenerating changes a tracked file (run it after committing) |
| `bun run dev:docker` | infra + apps in Docker with `compose.dev.yaml` (hot reload) |
| `bun run dev:hybrid` | `infra:up` then runs web (`bun --bun next dev`), api (`air -c .air.api.toml`), and worker (`air -c .air.worker.toml`) natively with prefixed, coloured logs. Ctrl+C stops all three. |
| `bun run dev:native` | the same as hybrid but skips Docker and checks that local Postgres, Redis, ffmpeg, and vips respond first, listing every missing one. Forces `STORAGE_DRIVER=fs`. Put native hosts/ports (e.g. `DATABASE_URL=postgres://…@localhost:5432/tepati`) in an optional, gitignored `.env.native`, which overrides `.env` in this mode only. |
| `bun run dev:apps` | only the three app processes (when infra is already running anywhere) |
| `bun run test` | Go unit tests and web unit tests |
| `bun run test:integration` | Go integration tests (`-tags=integration`) against the **running dev infra** (`bun run infra:up`): each test gets a throwaway Postgres database, and Redis tests use their own logical DB (auth 15, http 14, jobs 13). Add `-race` when running `go test` by hand |
| `bun run test:e2e` | Playwright against `dev:docker` with the test clock enabled |
| `bun run lint` | tsc `--noEmit` for scripts, Redocly lint of `api/openapi.yaml`, `gofmt -l`, `go vet` (eslint joins in PLAN 5.1) |
| `bun run go:tool <tool> …` | runs goose / sqlc / oapi-codegen pinned in `apps/server/tools/go.mod` (Go downloads the 1.26 toolchain for that module automatically) |
| `bun run deploy:build -- --env staging` | builds and tags images `tepati-web`/`tepati-server:<git sha>` |
| `bun run deploy:up -- --env staging` | `docker compose -f deploy/compose.yaml -f deploy/compose.prod.yaml --env-file deploy/env/.env.staging --profile infra --profile app --profile edge up -d`, then runs `migrate` |

Ports in dev: web `3000`, api `8080`, Postgres `55432` (Docker; 5432/5433 stay free for native Postgres installs), Redis `6379`, Garage S3 `3900` (admin `3903`), Mailpit UI `8025` (SMTP `1025`), asynqmon `8081`. Override them in `.env` if a port is taken.

### Windows and Go

If `go build` fails with `compile: version "go1.26.0" does not match go tool version "go1.25.3"`, the machine has an older system Go and the automatic toolchain switch is broken. Put the cached 1.26 toolchain first on `PATH` (path shown in `docs/STATUS.md §3`) and set `GOTOOLCHAIN=local`, or install Go 1.26 system-wide. `bun run lint|test|codegen` shell out to `go`, so they need it too.

`dev:hybrid` starts the api (`apps/server/.air.api.toml`) and the worker (`.air.worker.toml`); air builds into `apps/server/tmp/` (gitignored). The worker serves Prometheus metrics and `/healthz` on `WORKER_METRICS_PORT` (default 9091; `0` turns it off). That port, like the API's `/metrics`, must never be exposed publicly. Set `AIR_POLL=1` in `.env` to make air poll for changes (needed for bind mounts on Windows/macOS).

## 4. Compose structure and **mount rules**

- `deploy/compose.yaml` (base) defines every service with **profiles**: `infra` (postgres, redis, garage, mailpit), `app` (migrate, api, worker, web), `edge` (caddy), and `tools` (asynqmon).
- `deploy/compose.dev.yaml` adds source bind mounts and dev commands.
- `deploy/compose.prod.yaml` sets built images, `restart: unless-stopped`, resource limits, and no source mounts. Mailpit is replaced by real SMTP and asynqmon sits behind auth.

### Mount rules (read before touching compose)

1. **Database, Redis, and Garage data use named volumes only** (`pgdata`, `redisdata`, `garage-meta`, `garage-data`, `caddy-data`). Never bind-mount them to a host path. On Windows, bind mounts break Postgres permissions (`chmod` on NTFS), are slow, and risk corruption.
2. **Never mount host `node_modules` or `.next` into Linux containers.** In dev, mount `apps/web` and then shadow them with anonymous or named volumes:
   ```yaml
   volumes:
     - ../apps/web:/app/apps/web
     - web-node-modules:/app/node_modules
     - web-next:/app/apps/web/.next
   ```
   Host binaries (Windows/macOS) are not Linux binaries, so native modules would break.
3. **Go caches go into named volumes** (`go-mod:/go/pkg/mod`, `go-build:/root/.cache/go-build`) so rebuilds stay fast and nothing is written into the repo.
4. **File watching on Windows/macOS bind mounts needs polling.** Set `WATCHPACK_POLLING=true` for Next, and set `poll = true` with `poll_interval = 500` in `.air.*.toml` when `AIR_POLL=1`. The dev compose file sets both.
5. **Config files mount read-only**: `./garage/garage.toml:/etc/garage.toml:ro` and `./caddy/Caddyfile:/etc/caddy/Caddyfile:ro`.
6. **Relative paths in compose resolve from `deploy/`**, the compose file's directory. Always run compose with `-f deploy/...` from the repo root, or use the Bun scripts, which do this for you.
7. **Line endings:** `.gitattributes` forces `eol=lf` for `*.sh`, `*.toml`, `Caddyfile`, and Dockerfiles. CRLF in a shell entrypoint breaks containers with `exec format error` or `\r: not found`.
8. **Production images contain the built artefact only.** There are no source mounts and no dev volumes, and the containers run as non-root (`USER 10001`). Writable paths are tmpfs (`/tmp`) and the named volumes above.
9. **Uploads are never written to the container filesystem.** The worker streams staging→media through `/tmp` (tmpfs, sized 1 GB in prod) and cleans up in a `defer`.

## 5. Garage specifics

- The config is `deploy/garage/garage.toml`, single-node, with `replication_factor = 1`. `rpc_secret`, `admin_token`, and `metrics_token` come from env via `GARAGE_RPC_SECRET` etc. (Garage v2 supports `*_file` and env overrides; see the Garage docs via context7).
- The Garage image has no shell, so bootstrap is `scripts/garage-init.ts` (run by `infra:up` and by deploy scripts). It calls the CLI through `docker compose exec garage /garage …`: `node id`, `layout assign -z dc1 -c $GARAGE_CAPACITY` + `layout apply --version 1` (only while the layout version is 0), `key import` (only if the key is unknown), `bucket create` (only if missing), and `bucket allow` (idempotent). A re-run prints only `=` no-op lines.
- CORS on the `tepati-staging` bucket must allow `PUT` from `APP_BASE_URL` (set via the S3 `PutBucketCors` API in `garage:init`).
- In hybrid and native mode, the browser reaches Garage at `http://localhost:3900`. In Docker prod, Caddy exposes it at `https://media.<domain>` or under `/s3/*`, so set `S3_PUBLIC_ENDPOINT` accordingly.

## 6. Staging and production

- Staging and production use the same compose files with different env files: `deploy/env/.env.staging` and `.env.production`, created from the examples and **never committed**.
- `bun run deploy:up` order: infra (healthy), then `migrate` (one-shot, must exit 0), then api, worker, and web, then edge.
- Scale stateless services with `bun run deploy:scale -- api=3 web=2 worker=2`.
- Backups: a nightly `pg_dump` sidecar (`profile backup`) writes to the Garage bucket `tepati-backups` with 14 daily and 8 weekly copies. The restore procedure lives in `docs/RUNBOOK.md` (Phase 6).
- Zero-downtime deploys: migrations must be backward compatible (expand → deploy → contract). Pull new images, then `up -d` service by service, with Caddy health checks taking care of the switch.
