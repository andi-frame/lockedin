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
| PostgreSQL 18, Redis 7 or newer | native mode only | Windows: Postgres installer, and Redis via **Memurai** or WSL (Redis 6 is refused: the login limiter needs `EXPIRE ... NX`, from 7.0). Garage has no Windows build, so native mode uses `STORAGE_DRIVER=fs`. See section 7. |

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
| `bun run db:seed [-- overdue\|invite\|today]` | `tepatictl seed --scenario …`. `overdue` (default): an active pact that started 3 days ago with check-ins past their deadline, which a running worker marks `missed` within a minute or two. `invite`: a proposed pact with an open invite link; with the worker running, the invite email to `seed-invitee@tepati.test` shows up in Mailpit (`http://localhost:8025`) within seconds. Both reuse the users `seed-backer@tepati.test` and `seed-doer@tepati.test` (password `tepati-seed-1234`) and print the pact id. `today`: new users on every run (`today-<tag>-doer@tepati.test`, same password) with five active pacts whose check-ins dated today are open (two of them, one with evidence rules: 10 words and one ready attachment), submitted, approved and missed; it is what `today.spec.ts` and `proof.spec.ts` sign in to |
| `bun run ctl -- <args>` | `tepatictl` with `.env` loaded, for example `bun run ctl -- pact show <id>` (members, every check-in, the ledger, the balance). `bun run ctl -- seed --scenario passbook` makes new users (`passbook-<tag>-doer@tepati.test`) and one active pact with 24 days of printed lines (33 in all) and today still open for the doer. `bun run ctl -- seed --scenario review` makes a backer (`review-<tag>-backer@tepati.test`) with three pacts for the review flows: two submitted proofs and one auto-approved inside its override window. `bun run ctl -- seed --scenario review` leaves notifications behind (the worker's outbox relay turns them into inbox rows, so the worker must run). `bun run ctl -- seed --scenario settlement` makes a backer and a doer (`settlement-<tag>-*@tepati.test`) with two pacts that are in `settling` with 900 coins owed. `bun run ctl -- advance --pact <id>` is the test clock: it moves a clock to one second past that pact's next open deadline and ticks only that pact's check-ins with the real service code, so the miss is printed as the worker would print it (a running API shows it on the next 30 s refresh) |
| `bun run codegen` | sqlc, then OpenAPI → Go strict server (`internal/http/api`) and web types (`apps/web/src/lib/api/schema.d.ts`). `-- --check` fails if regenerating changes a tracked file (run it after committing) |
| `bun run dev:docker` | infra + apps in Docker with `compose.dev.yaml` (hot reload) |
| `bun run dev:hybrid` | `infra:up` then runs web (`bun --bun next dev`), api (`air -c .air.api.toml`), and worker (`air -c .air.worker.toml`) natively with prefixed, coloured logs. Ctrl+C stops all three. |
| `bun run dev:native` | the same as hybrid but skips Docker and checks that local Postgres, Redis, ffmpeg, and vips respond first, listing every missing one. Forces `STORAGE_DRIVER=fs`. Put native hosts/ports (e.g. `DATABASE_URL=postgres://…@localhost:5432/tepati`) in an optional, gitignored `.env.native`, which overrides `.env` in this mode only. |
| `bun run dev:apps` | only the three app processes (when infra is already running anywhere) |
| `bun run test` | Go unit tests and web unit tests |
| `bun run test:integration` | Go integration tests (`-tags=integration`) against the **running dev infra** (`bun run infra:up`): each test gets a throwaway Postgres database, and Redis tests use their own logical DB (auth 15, http 14, jobs 13). Add `-race` when running `go test` by hand |
| `bun run test:e2e` | Playwright specs in `apps/web/tests/e2e` against a stack that is already running (`bun run dev:hybrid`; the script checks web and API first and extra arguments go to Playwright). It seeds the today scenario first (one doer per spec and per project). The proof spec needs the media worker, ffmpeg and libvips. The API allows `AUTH_RATE_LIMIT_PER_MIN` auth requests a minute per IP (default 10, at most 10 in production); a full run needs about 11, so put `AUTH_RATE_LIMIT_PER_MIN=200` in your local `.env`, or run the specs in two goes a minute apart. If port 8080 is busy, set `E2E_API_URL` to wherever the API listens. There is no test clock; time-dependent states come from the seed |
| `bun run lint` | tsc `--noEmit` for scripts, Redocly lint of `api/openapi.yaml`, `gofmt -l`, `go vet` and, in `apps/web`, `tsc --noEmit` plus ESLint |
| `bun run go:tool <tool> …` | runs goose / sqlc / oapi-codegen pinned in `apps/server/tools/go.mod` (Go downloads the 1.26 toolchain for that module automatically) |
| `bun run deploy:build -- --env staging` | builds `tepati-server` (api, worker, tepatictl, goose, migrations) and `tepati-web`, each tagged `<env>` and with the `git describe` version; `--only server\|web` builds one |
| `bun run load -- --env staging [--users 20] [--duration 5m] [--read-rate 200] [--write-rate 20]` | the k6 load test (`tests/load/today.js`) against a running deploy: seeds users through its api container and runs k6 from the `grafana/k6` image on the deploy's network; exits non-zero when a SPEC §10 threshold is crossed. Needs `RATE_LIMIT_PER_MIN` in the env file above the test's own rate (it checks) |
| `bun run restore -- --env staging list \| check <key> \| restore <key> --live` | the Postgres backups in the Garage bucket: list them, test one in a scratch database, or replace the live database (docs/RUNBOOK.md §6) |
| `bun run deploy:env-local` | writes `deploy/env/.env.staging` for a deploy on this machine or a CI runner (`https://localhost`, Mailpit, fresh secrets, raised rate limits); refuses to overwrite a file |
| `bun run deploy:up -- --env staging` | checks `deploy/env/.env.<env>` (no `CHANGE_ME`, `S3_PUBLIC_ENDPOINT` on `media.<DOMAIN>`) and the images, brings up infra, sets up Garage (layout, key, buckets; generates the S3 key pair into the env file the first time), then migrate, api, worker, web and Caddy, then the CORS rule for uploads. Compose project `tepati-<env>`. `--backup` / `--no-backup` (default: on for production) |
| `bun run deploy:scale -- --env staging api=2 web=2 worker=2` | changes the number of copies of the stateless services; nothing else is recreated, and Caddy finds new api and web copies through DNS |

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
- In hybrid and native mode, the browser reaches Garage at `http://localhost:3900`. In a deploy, Caddy exposes it at `https://media.<DOMAIN>` (a separate host, not a `/s3` prefix: a presigned URL signs its path, so a prefix stripped by the proxy would break it), so `S3_PUBLIC_ENDPOINT=https://media.<DOMAIN>`. It needs its own DNS record.
- A deploy's Garage S3 port is not published, so `deploy:up` sets the bucket CORS rule from inside the network with `tepatictl storage-init`.

## 6. Staging and production

- Staging and production use the same compose files (`deploy/compose.yaml` plus `deploy/compose.prod.yaml`) with different env files: `deploy/env/.env.staging` and `.env.production`, copied from the examples by hand, with every `CHANGE_ME` replaced, and **never committed**. Secrets: `bun scripts/setup.ts --print-secret hex32`. Leave `S3_ACCESS_KEY` and `S3_SECRET_KEY` empty: the first `deploy:up` generates them into the file. `TEPATI_ENV` (set by the scripts) picks the env file and the image tag; each environment is its own compose project (`tepati-staging`, `tepati-production`), so it never shares volumes with the dev infra.
- First deploy of an environment: `bun run deploy:build -- --env staging`, then `bun run deploy:up -- --env staging`. The order inside `deploy:up`: infra (healthy), Garage layout, key and buckets (the API is not ready without its buckets), then `migrate` (one-shot, must exit 0), api, worker and web, then Caddy, then `tepatictl storage-init` for the upload CORS rule. Run it again for an update: unchanged services stay, changed images are recreated, and migrations run first.
- DNS: `<DOMAIN>` and `media.<DOMAIN>` both point at the host; ports 80 and 443 must be reachable (Caddy gets the certificates by itself). On a laptop use `DOMAIN=localhost`: Caddy then issues certificates from its own CA, which browsers do not trust (the e2e config ignores that for `https://localhost`).
- Mail: staging keeps Mailpit (`SMTP_URL=smtp://mailpit:1025`, UI on the container network only; `docker compose -p tepati-staging exec ...` or add a port in a local override); production needs a real `SMTP_URL`. `deploy:up` switches the Mailpit profile on only when `SMTP_URL` points at it.
- Scale stateless services with `bun run deploy:scale -- --env staging api=3 web=2 worker=2` (1 to 20 each). Several workers are safe: the periodic tasks are unique and the sweeps use `SKIP LOCKED`.
- Edge: Caddy serves `<DOMAIN>` (web, `/api/*` to the API with a 2 MB body cap, `/healthz` and `/readyz` for an uptime monitor) and `media.<DOMAIN>` (Garage, 210 MB body cap), adds HSTS, a CSP, `nosniff`, `X-Frame-Options: DENY` and a referrer and permissions policy. The CSP keeps `'unsafe-inline'` for scripts and styles because Next.js writes inline ones.
- Queue dashboard: `docker compose -p tepati-<env> ... --profile tools up -d asynqmon` starts asynqmon on `127.0.0.1:${ASYNQMON_PORT:-8081}` only (it has no login). On a server, reach it through an SSH tunnel.
- Backups (profile `backup`): a nightly `pg_dump` (custom format) at `BACKUP_HOUR_UTC` (3) into the Garage bucket `tepati-backups` as `daily/YYYY-MM-DD.dump`, plus `weekly/YYYY-Www.dump` on Sundays, keeping 14 and 8 (`BACKUP_KEEP_DAILY`, `BACKUP_KEEP_WEEKLY`, at least 1). A first backup right away: `docker compose -p tepati-<env> -f deploy/compose.yaml -f deploy/compose.prod.yaml --env-file deploy/env/.env.<env> --profile infra --profile backup run --rm -T -e BACKUP_ONCE=1 backup`. `deploy:up` includes the profile for production by default.
- Restore: `bun run restore -- --env <env> list`, `check <key>` (the drill: a scratch database, nothing live is touched) and `restore <key> --live` (replaces the live database after a safety copy; stop the api and the worker first). Details, the rehearsal and the way back are in `docs/RUNBOOK.md` section 6.
- Rate limits: `RATE_LIMIT_PER_MIN` (all API calls per client IP, default 300) and `AUTH_RATE_LIMIT_PER_MIN` (default 10, at most 10 in production). The web server forwards each visitor's address, so the budgets are per visitor, not per web container.
- Zero-downtime deploys: migrations must be backward compatible (expand, deploy, contract). Scale api and web to 2 or more, then update the images one service at a time; Caddy re-resolves the copies every 5 seconds.
- Run the e2e suite against a deploy: `TEPATI_CTL_PROJECT=staging E2E_BASE_URL=https://localhost E2E_API_URL=https://localhost bun run test:e2e`. The seeds run inside that deploy's api container. Put `AUTH_RATE_LIMIT_PER_MIN=200` and `RATE_LIMIT_PER_MIN=3000` in its env file for the run (staging only; never in production).
- `bun run dev:docker`, `deploy:up` and `deploy:scale` run with `bun --no-env-file`: Bun would otherwise load the root `.env` into the environment, and compose gives the environment priority over `--env-file`, so hybrid values like `API_PORT` would leak into the containers' definitions. If port 8080 is busy, set `API_HOST_PORT` in `deploy/env/.env.dev`.

## 7. Native mode (no Docker)

For a machine where Docker is not an option. Postgres and Redis run on the host, files go to the `fs` driver (`FS_STORAGE_DIR`, under `apps/server/.data/blobs`), and mail goes to whatever `SMTP_URL` says (no Mailpit unless you run one).

1. Install PostgreSQL (18 recommended) and create an empty database and a login for it. Install Redis 7 or newer: **Memurai** on Windows, or Redis 7/8 in WSL (it listens on `localhost`, which Windows reaches; expect each Redis round trip to cost a few tens of milliseconds, so the app feels slower than with Memurai or Docker). ffmpeg and libvips as in section 1.
2. Copy `deploy/env/.env.native.example` to `.env.native` in the repo root (gitignored) and set `DATABASE_URL` and `REDIS_URL`; add ports only if something else holds the defaults. It is layered over `.env`, and `STORAGE_DRIVER` is always `fs`.
3. `bun run dev:native` checks everything first and lists **every** problem at once: a real Postgres login (a wrong password or a missing database says so, with the `createdb` command), Redis version, ffmpeg, ffprobe, vips, vipsheader. Then it applies the pending migrations to the native database and starts web, api and worker. Source `scripts/dev-env.sh` first in a shell opened before the ffmpeg and vips install.
4. Next.js allows one dev server per project folder, so stop `dev:hybrid` or `dev:docker` first (or run native from another checkout). Docker containers may stay up; use other ports in `.env.native` if they hold the defaults.
5. `TEPATI_NATIVE=1` makes `bun run db:migrate`, `bun run ctl` and the e2e seeds use `.env.native` (and its database) instead of the Docker one: `TEPATI_NATIVE=1 bun run ctl -- seed --scenario today`. For the e2e on a slow stack: `TEPATI_NATIVE=1 E2E_BASE_URL=http://localhost:3010 E2E_API_URL=http://localhost:18090 E2E_TIMEOUT_MS=120000 E2E_EXPECT_TIMEOUT_MS=30000 bun run test:e2e`.
6. A throwaway Postgres for a check, without touching an installed server (Windows, PowerShell or Git Bash): `initdb -D .data/pg-native -U tepati --auth=trust`, `pg_ctl -D .data/pg-native -o "-p 5440" -l .data/pg-native.log start`, then `psql -h localhost -p 5440 -U tepati -d postgres -c "create database tepati"` and `DATABASE_URL=postgres://tepati@localhost:5440/tepati?sslmode=disable` in `.env.native`. Stop it with `pg_ctl -D .data/pg-native stop`. `.data/` is gitignored.

## 8. CI

`.github/workflows/ci.yml` runs on every pull request and every push to `main`, in three jobs:

| Job | Commands | Notes |
|---|---|---|
| `check` | `bun run lint`, `bun run codegen -- --check`, `bun run test` | installs ffmpeg and libvips so the media golden tests run instead of skipping; the codegen check fails if regenerating sqlc, the Go server or the web types changes a tracked file |
| `integration` | `bun run setup`, `bun run infra:up`, `bun run test:integration` | the same Docker infra as local development (Postgres, Redis, Garage, Mailpit) |
| `e2e` | `deploy:env-local`, `deploy:build -- --env staging`, `deploy:up -- --env staging`, `test:e2e` with `TEPATI_CTL_PROJECT=staging E2E_BASE_URL=https://localhost E2E_API_URL=https://localhost` | the built images behind Caddy, seeded through the api container; uploads `apps/web/test-results` and the stack logs on failure; Playwright retries once on CI and reports a retried test as flaky |

**Docker Hub secrets.** The `integration` and `e2e` jobs pull `postgres`, `redis`, `golang` and other base images, and Docker Hub limits anonymous pulls per IP while GitHub's runners share IPs, so a run can fail with `toomanyrequests` before it tests anything. Both jobs sign in with `docker/login-action` when the repository secrets `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` exist (a free Docker Hub account and a personal access token with read-only access are enough). Without the secrets, for example in a fork's pull request, the step is skipped and the pulls stay anonymous; if Docker Hub's auth endpoint itself times out, the step is allowed to fail and the pulls fall back to anonymous. The same two jobs also point the runner's Docker daemon at `https://mirror.gcr.io`, so most base images do not come from Docker Hub at all (an image the mirror lacks falls back to Docker Hub). If a run fails with `toomanyrequests`, check that both secrets are set and that the token has not expired.

To reproduce the `e2e` job on a laptop, run those four commands in order (delete `deploy/env/.env.staging` first if it exists, and `docker compose -p tepati-staging ... down -v` if an older stack left volumes behind, because the new secrets will not match the old database).

## 9. Load test

`bun run load -- --env staging` needs a running deploy (section 6) and Docker; k6 itself is not installed, it runs from `grafana/k6`. It seeds `--users` doers (the `today` scenario) through the deploy's api container, logs them in, and holds `--read-rate` reads a second (Today, pacts list, a pact's ledger, notifications, review queue) and `--write-rate` proof edits a second for `--duration`. The thresholds are SPEC section 10: read p95 under 150 ms, write p95 under 300 ms, under 1 % failed requests.

Before you run it: every request comes from the one k6 address, so raise `RATE_LIMIT_PER_MIN` in the env file above `(read-rate + write-rate) * 60 * 1.2` (for the defaults, 15,840; the script refuses to run below it) and bring the deploy up again so the api picks it up. Stop other heavy things on the machine (the dev stack, a browser): they compete for the CPU and the numbers are only as good as the machine. To see what a second api copy buys, run `bun run deploy:scale -- --env staging api=2` and repeat.

Results of the first run, and why they are not the SPEC verdict for a 2 vCPU server, are in `docs/PLAN.md` 8.4.
