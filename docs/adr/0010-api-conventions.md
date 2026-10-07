# ADR-0010: API conventions: errors, idempotency, bodies, pagination

Status: Accepted (2026-10-07)

## Context
Phase 2 turned `api/openapi.yaml` into a running API (Fiber v3, oapi-codegen strict server). Building it forced a set of choices that every client, and every later phase, depends on. Some came from limits of the generators, and a few from bugs found while testing.

## Decision

**Errors.** Every error is RFC 9457 `application/problem+json` with a stable `code`. Clients branch on `code`, never on `title` or `detail`. The status for each code is a table inside the `ErrorCode` schema of the contract; `internal/http/statuses.go` mirrors it, and `spec_test.go` fails when the two differ, or when a Go service returns a code the contract does not list. Unknown errors become `500 server.internal` and their cause goes to the logs only. A non-member gets `404 not_found`, indistinguishable from an unknown id, so pact existence never leaks.

**Contract first, generated code never edited.** `api/openapi.yaml` → `bun run codegen` → Go strict server in `internal/http/api` and `apps/web/src/lib/api/schema.d.ts`. CI-style check: `bun run codegen -- --check`. Handlers only translate and call one service function; rules live in `internal/service` and `internal/domain`.

**Idempotency.** Mutating requests may carry `Idempotency-Key` (8–128 characters). The key is scoped by user, method and path; the first 2xx response is stored in Redis for 24 hours and replayed with `Idempotent-Replayed: true`. The same key with a different request is `422 idempotency.key_reused`; the same key while the first request is still running is `409 idempotency.in_progress`. Non-2xx responses are not stored, so a retry runs again (safe because every transition is one transaction). If Redis is down the middleware fails open: the database's own idempotency keys and expected-status guards still protect the ledger.

**Request bodies are JSON only.** An empty body on POST, PUT or PATCH counts as `{}`, because the generated handlers bind a body even where the contract says it is optional and fail on an empty one. Any other content type is `415`. This also means a cross-site `<form>` post cannot reach a handler.

**Authentication and CSRF.** Session cookie (`tepati_session`, HttpOnly) plus a readable `tepati_csrf` cookie whose value the client sends as `X-CSRF-Token` on every unsafe method. The public routes (register, login, invite preview) are listed in code and tied to the contract's `security: []` by a test.

**Pagination.** `cursor` (opaque base64url JSON keyset) and `limit` (1–100, default 30); responses carry `next_cursor`, `null` on the last page. A forged cursor can move the window but never widen what a user sees, because every query is already filtered to the caller. The passbook's `balance_after` is a window function over the whole ledger, so it agrees across pages.

**The server tells the client what it may do.** `GET /check-ins/{id}` returns `my_actions`, computed by dry-running `domain.Transition` for each action. The web app renders buttons from that list and does not re-derive the state machine.

**Limits.** 300 requests per minute per IP globally, 10 on `/auth/*`, 30 on upload intents, counted in Redis. Health and metrics endpoints are exempt. Client IP comes from `X-Forwarded-For` only when the peer is loopback or private.

**Ops endpoints** `/healthz`, `/readyz`, `/metrics` sit at the origin root, outside `/api/v1`, and are excluded from the generated code. `/metrics` must not be exposed publicly (Caddy, PLAN 7.2).

## Consequences
- Adding an error code is a contract change first: enum, status table, `statuses.go`, regenerate. The spec tests tell you what you forgot.
- The web app can show a precise message for every failure and never needs to parse text.
- Because empty bodies become `{}`, a handler with a required body must validate its fields (they arrive as zero values); every handler does.
- Retried submissions are safe, but a replayed response can show state from before later changes; clients should refetch after a replay if they need the current state.
- Per-field validation errors (`problem.errors[]`) are in the contract but not yet produced; the wizard in Phase 6.1 will need `Terms.Validate` to return structured errors.
- Fiber v3 specifics that bit us are recorded in `docs/STATUS.md §5`.
