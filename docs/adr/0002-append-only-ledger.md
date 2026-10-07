# ADR-0002: Append-only ledger with idempotency keys

Status: Accepted (2026-10-07)

## Context
The pot balance changes because of time-based events, such as missed days, that background workers process. Those workers may retry or run concurrently. A mutable `balance` column would invite drift and double-charging.

## Decision
- `ledger_entries` is append-only, enforced by a database trigger. The balance is `SUM(amount)`.
- Every economic event has a deterministic `idempotency_key` backed by a unique constraint.
- Floor and cap clamps are computed in the same transaction, under `SELECT … FOR UPDATE` on the pact row.
- Corrections are written as new `reversal` rows.

## Consequences
- The passbook UI renders the ledger directly, with a running balance computed by a window function.
- Settlement can be replayed safely with `tepatictl settle --replay`.
- At most 366×2 rows exist per pact, so `SUM` stays cheap. Cache the balance in Redis (30 s TTL plus invalidation on write) when read volume grows.
