# Runbook: running Tepati on a server

For whoever operates a staging or production deploy. It assumes one Linux host with Docker, which is what `deploy/compose.yaml` plus `deploy/compose.prod.yaml` describe. Everything here was exercised **on a laptop** with `DOMAIN=localhost` (PLAN 7.2, 8.4, 9.1); the server preparation in section 2 is written from standard practice and **has not been rehearsed on a real VPS**. Where a command is only known from the laptop it says so.

The run commands are in `docs/RUNNING.md`; this file is the order to do things in, and what to do when something goes wrong.

## 1. What runs, and what you need before the first deploy

One compose project per environment (`tepati-staging`, `tepati-production`), started with `bun run deploy:up -- --env <env>`:

| Container | What it is | Reachable from outside |
|---|---|---|
| `caddy` | TLS, one origin for the app and the API, the media host | ports 80 and 443 |
| `web` | the Next.js standalone server | no (through Caddy) |
| `api` | the Go API (`/api/*`, `/healthz`, `/readyz`) | through Caddy |
| `worker` | settlement, reminders, email, media processing | no |
| `postgres`, `redis`, `garage` | data (named volumes) and files | no |
| `mailpit` | staging only, when `SMTP_URL` points at it | no |
| `backup` | nightly `pg_dump` to the Garage bucket | no |

**Decisions that belong to the owner** (nothing below can be done without them):

- A host: Linux, Docker Engine with the Compose plugin, at least 2 vCPU and 4 GB. The load test (PLAN 8.4) put one api container at about 60 % of one CPU for 220 requests a second on a fast laptop, with Postgres at 40 %, so 2 vCPU leaves room; it is not a measurement of any particular VPS.
- A domain, and control of its DNS. **Two names must point at the host:** `<DOMAIN>` and `media.<DOMAIN>` (a presigned upload signs its path, so the media host cannot be a path prefix). Ports 80 and 443 must be open to the internet from the start: Caddy gets the certificates by itself, and it needs port 80 for that.
- An SMTP server and a sender address, for production. Staging can keep Mailpit (no mail leaves). A real sender needs SPF and DKIM set on its domain or the mail will land in spam.
- Who may log in to the server, and where the backups are copied to (see section 6: the dumps live in the same Garage as the data, so a lost disk loses both unless they are copied elsewhere).

## 2. Prepare the server (not rehearsed on a VPS)

1. Update it, create a non-root user with your SSH key, switch off password login (`PasswordAuthentication no`) and root login, and enable automatic security updates (`unattended-upgrades` on Debian and Ubuntu).
2. Firewall: allow 22, 80, 443 (TCP) and 443 (UDP, for HTTP/3), deny the rest. Docker publishes ports around `ufw`, but only Caddy publishes any port in this setup (`docker ps` should show nothing else with `0.0.0.0`).
3. Install Docker Engine and the Compose plugin from Docker's own repository, and add your user to the `docker` group. Cap the container logs in `/etc/docker/daemon.json`: `{"log-driver": "json-file", "log-opts": {"max-size": "20m", "max-file": "5"}}`, then restart Docker.
4. Install Bun (`curl -fsSL https://bun.sh/install | bash`) and Git. The deploy scripts are Bun scripts and run on the server.
5. Get the code: `git clone <repo> && cd lockedin` (a private repository needs a read-only deploy key). Deploy a tag or a commit you have seen pass CI, not a moving branch: `git checkout <commit>`.
6. Set the time zone to UTC and check `timedatectl` says the clock is synchronised: the worker's deadlines compare with the server's clock.

## 3. First deploy

1. `cp deploy/env/.env.staging.example deploy/env/.env.staging` (or `.env.production.example`). Replace every `CHANGE_ME`. Secrets: `bun scripts/setup.ts --print-secret hex32` for each. Leave `S3_ACCESS_KEY` and `S3_SECRET_KEY` empty: the first `deploy:up` generates them and writes them into the file. Keep the file out of git (it is ignored) and back it up somewhere safe: **losing it means losing the database password and the object-store keys**.
2. `S3_PUBLIC_ENDPOINT` must be `https://media.<DOMAIN>`, `APP_BASE_URL` must be `https://<DOMAIN>`, `ACME_EMAIL` must be a real address. `deploy:up` checks all of this, and refuses `CHANGE_ME`, `localhost` in production, Mailpit in production, and a raised auth rate limit in production, before it touches anything.
3. `bun run deploy:build -- --env <env>` builds the images (it pulls fresh base images, so the Go toolchain is the current patch release).
4. `bun run deploy:up -- --env <env>`. It brings up the data services, sets Garage up (layout, key, buckets), runs the migrations, starts the app and Caddy, and sets the bucket CORS rule. The first request to `https://<DOMAIN>` can take a few seconds while the certificate is issued.
5. Check it from **outside** the server, not from the server:
   - `curl -sI https://<DOMAIN>/` shows the security headers and no `Server` header;
   - `curl -s https://<DOMAIN>/readyz` says `database`, `redis` and `storage` are `ok`;
   - open the site, register, create a pact, invite a second account (use a private window), submit a proof with a photo (this goes through `media.<DOMAIN>`);
   - `curl -s https://<DOMAIN>/metrics` must **not** return metrics (it is not routed, and the worker's port is not published).
6. Take the first backup right away and run the restore check (section 6) so you know the backups work before you need them.
7. Put an uptime monitor on `https://<DOMAIN>/readyz` (it shows only whether the database, Redis and storage answer).

Staging with Mailpit: the container is reachable only on the compose network. To read the mail, `docker compose -p tepati-staging ... port` is not published; either run a second Caddy site for it behind a login, or temporarily publish `127.0.0.1:8025` with a local override and use an SSH tunnel.

## 4. Updating

1. `git fetch && git checkout <new commit>`; read the PLAN and STATUS notes for anything that changed in the env file (`deploy:up` will name a missing or wrong value).
2. `bun run deploy:build -- --env <env>`, then `bun run deploy:up -- --env <env>`. Unchanged services stay; changed images are recreated; **migrations run first** (the `migrate` one-shot must exit 0, or the app containers do not start).
3. Migrations must be backward compatible with the version that is still running (expand, deploy, contract). A migration that is not (a dropped or renamed column) needs a maintenance window: stop `api` and `worker`, deploy, start them.
4. For no downtime on a single host, scale first: `bun run deploy:scale -- --env <env> api=2 web=2`, then update; Caddy finds the new containers through DNS within 5 seconds. Scale back afterwards.
5. Check `/readyz` and the e2e smoke you have (`bun run test:e2e` against the host works with a seeded user only if you run the seeds on the host: `TEPATI_CTL_PROJECT=<env>`; on production prefer a manual check, the seeds create users).

## 5. Rolling back

- **Code:** every build is tagged with its version (`docker image ls | grep tepati` shows `tepati-server:<describe>` next to `tepati-server:<env>`). To go back: `docker tag tepati-server:<old> tepati-server:<env>` and the same for `tepati-web`, then `bun run deploy:up -- --env <env>` (it does not rebuild). Check out the matching commit too, so the compose files and the images agree.
- **Database:** migrations are not reversed automatically. If a migration has to be undone, restore the backup taken before the update (section 6), accepting the loss of what was written since. Take a manual backup (`BACKUP_ONCE=1`, section 6) right before every update that has a migration.

## 6. Backups and the restore drill

What exists: the `backup` container writes a `pg_dump` (custom format) to the Garage bucket `tepati-backups` every night at 03:00 UTC: `daily/YYYY-MM-DD.dump` (the newest 14 are kept) and, on Sundays, `weekly/YYYY-Www.dump` (the newest 8). It is part of `deploy:up` for production (`--backup` for staging).

**Limits you should know:** the bucket is in the same Garage as the uploads, on the same disk as Postgres. This protects against a bad migration, a bad deploy and an operator mistake; it does **not** protect against losing the host. Copy the dumps off the machine (a scheduled `rclone` or `aws s3 sync` to another provider, or a second backup job), and copy the env file. The uploaded files themselves (`garage-data`) are not in the dumps: back up that volume separately if losing uploads matters.

Commands (all run from the repository on the server):

| What | Command |
|---|---|
| Take a backup now | `docker compose -p tepati-<env> -f deploy/compose.yaml -f deploy/compose.prod.yaml --env-file deploy/env/.env.<env> --profile infra --profile backup run --rm -T -e BACKUP_ONCE=1 backup` (with `TEPATI_ENV=<env>` set) |
| List backups | `bun run restore -- --env <env> list` |
| **The drill:** restore a backup into a scratch database, count what is in it, drop it | `bun run restore -- --env <env> check daily/2026-10-09.dump` |
| Replace the live database | stop `api` and `worker`, then `bun run restore -- --env <env> restore daily/2026-10-09.dump --live`, then start them |
| Undo a restore | the same command with the `pre-restore/...` key the restore printed |

`check` touches nothing live: it downloads the dump, checks it is a readable archive, restores it into a database called `tepati_restore_check`, prints the migration version and row counts (users, pacts, check-ins, proofs, ledger entries, decisions) and how many pots do not add up (must be 0), and drops it. `restore --live` first takes a copy of the live database into `pre-restore/<time>.dump` and only then replaces it; if the copy fails, nothing is replaced.

**The drill was rehearsed on 2026-10-09 on a local staging stack** (not on a server): a backup of 250 users, 625 pacts and 750 ledger entries was restored into the scratch database and reported the same counts and zero pots that do not add up; a user registered after the backup was gone after `restore --live` (251 users to 250) and was back after restoring the `pre-restore` copy (250 to 251); the app was ready after each. A wrong key is refused with the storage's own error, `restore` without `--live` is refused, and `--live` is not accepted with `check`. **Do the drill once on the real server before launch, and again after any change to the backup setup**, and write the date here.

Restoring after losing the host: install Docker, restore the env file, `deploy:up` (this creates empty services), copy a dump into the bucket (or into the `backup` container) and `restore ... --live`. Uploads come back only if you restored `garage-data` too.

## 7. Looking at it

- Logs: `docker compose -p tepati-<env> -f deploy/compose.yaml -f deploy/compose.prod.yaml --env-file deploy/env/.env.<env> --profile infra --profile app --profile edge logs -f --tail 100 api worker` (with `TEPATI_ENV=<env>` set). They are JSON, with a `request_id` that is also in the `X-Request-ID` response header; proof text and tokens are not logged.
- Health: `docker ps` (the api, worker and web have health checks), `https://<DOMAIN>/readyz`.
- Metrics (Prometheus format) are internal on purpose: `docker run --rm --network tepati-<env>_default curlimages/curl -s http://api:8080/metrics`, and the worker's on `http://worker:9091/metrics`. Do not publish these ports.
- The queue dashboard: `--profile tools up -d asynqmon` listens on `127.0.0.1:8081` of the server only, with no login; reach it with `ssh -L 8081:127.0.0.1:8081 <server>`. Stop it when you are done.
- Admin commands: `docker compose ... exec api tepatictl pact show <id>` (members, check-ins, ledger). `tepatictl seed` and `advance` are for development and staging; do not run them on production.

## 8. Secrets

| Secret | What changing it does |
|---|---|
| `SESSION_SECRET` | signs session lookups and CSRF tokens: **every user is signed out**. Change it, `deploy:up`. |
| `POSTGRES_PASSWORD` | the database keeps its old password (the variable only sets it when the volume is created): change it in Postgres first (`ALTER USER tepati PASSWORD '...'`), then in the env file, then `deploy:up`. Changing only the file locks the app out. |
| `S3_ACCESS_KEY`, `S3_SECRET_KEY` | create a new key in Garage (`garage key create`), allow it on the three buckets, put it in the env file, `deploy:up`, then delete the old key. |
| `GARAGE_*` tokens | recreated on restart; change in the env file and `deploy:up`. |
| SMTP credentials | change `SMTP_URL`, `deploy:up`. |

## 9. When something is wrong

| Symptom | Likely cause and what to do |
|---|---|
| `deploy:up` refuses with a list | the env file is not fit to deploy: fix what it names. |
| The certificate is not issued; the browser warns | `<DOMAIN>` or `media.<DOMAIN>` does not resolve to this host yet, or port 80 is closed; `docker logs <project>-caddy-1` says which. DNS can take time; Let's Encrypt rate-limits repeated failures. |
| The site is up but uploads fail in the browser | the media host does not resolve or has no certificate, or the bucket CORS rule is for another origin (after a domain change, run `deploy:up` again: it sets it). |
| `readyz` says `storage: down` | Garage is down or the bucket or key is missing; `deploy:up` again repairs the bucket and key. |
| Everyone gets 429 | the per-IP limits (`RATE_LIMIT_PER_MIN`, `AUTH_RATE_LIMIT_PER_MIN`): behind Caddy the API sees each visitor's address through `X-Forwarded-For`; if you put another proxy or a CDN in front, Caddy would see only its address, and every visitor shares one budget. |
| Login says "too many attempts" | 10 attempts a minute per IP and per email: wait a minute. In production this limit cannot be raised. |
| The app cannot connect to Postgres after an env change | the password in the file no longer matches the database's (section 8). |
| A compose command complains about a missing file or variable | run it with `TEPATI_ENV=<env>` set and `--env-file deploy/env/.env.<env>`, from the repository root; the Bun scripts do all of this for you. |
| Disk fills up | check `docker system df`, the log sizes (section 2), and the `garage-data` volume; old images go with `docker image prune`. |
| A deadline looks late or early | check the server clock (`timedatectl`); deadlines use the server's clock and the pact's time zone. |

## 10. Known gaps

- The server does not compare a pact's `starts_on` with today's date, so a pact proposed for a date and signed after it starts with overdue check-ins (STATUS §6).
- Changing the e-mail address and the password is not possible from the app (name, language, time zone and which e-mails to get are, in Settings).
- The CSP keeps `'unsafe-inline'` for scripts and styles because Next.js writes inline ones; a per-request nonce is the next hardening step.
- Dumps and uploads live on one host until you copy them elsewhere (section 6).
