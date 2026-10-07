# ADR-0004: asynq on Redis for jobs and scheduling

Status: Accepted (2026-10-07)

## Context
We need:
- deadline sweeps every minute
- media transcoding outside the request path
- retries, priorities, and the ability to absorb bursts

Redis is already required for sessions, rate limits, and caching.

## Options
1. **asynq** (a Go library backed by Redis). It provides retries with backoff, delayed and periodic tasks, unique tasks, weighted priority queues, and an inspector with a web UI (asynqmon).
2. **Raw Redis Streams with consumer groups.** Flexible, but we would have to build retries, delays, dead letters, and scheduling ourselves.
3. **NATS JetStream or RabbitMQ.** Solid brokers, but each is one more stateful service to run in every environment, including native Windows dev.
4. **A Postgres-backed queue (River).** Transactional enqueue is attractive, but it puts load on the database of record, and the owner asked for Redis.

## Decision
Use asynq with three queues:

| Queue | Used for | Weight |
|---|---|---|
| `critical` | settlement | 6 |
| `default` | notifications and email | 3 |
| `media` | transcoding | 1, plus its own concurrency cap |

`cmd/worker` registers periodic tasks with `asynq.Scheduler` (the schedule is a fixed table in `internal/jobs`, so `PeriodicTaskManager`'s dynamic reloading is not needed). Every handler is idempotent.

## Consequences
- Redis must run with AOF persistence (`appendonly yes`) so queued tasks survive a restart.
- Side effects that must be emitted exactly at commit go through the outbox table.
- asynq has no per-queue concurrency, so the `media` cap (2) is enforced inside the media handler, not by the server (implemented with PLAN 4.2).
- Periodic tasks are enqueued with `MaxRetry(0)` and a unique lock slightly shorter than their period: two worker replicas schedule the same tick once, and a failed tick is repaired by the next one, because every job re-derives its work from the database.
- If asynq stops being maintained, River (Postgres) is the fallback. Task handlers sit behind our own `jobs` interfaces, so swapping is contained.
