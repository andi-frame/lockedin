# Architecture Decision Records

ADRs are short, immutable records of decisions. To change a decision, add a new ADR that supersedes the old one. Don't edit the old one, except to set `Status: Superseded by ADR-XXXX`.

Format: Context → Decision → Consequences. Use the next free number.

| # | Title | Status |
|---|---|---|
| [0001](0001-iou-ledger.md) | Coins are an IOU ledger; no money flows through Tepati (MVP) | Accepted |
| [0002](0002-append-only-ledger.md) | Append-only ledger with idempotency keys | Accepted |
| [0003](0003-stack.md) | Go + Fiber API, Next.js on Bun, spec-first OpenAPI | Accepted |
| [0004](0004-queue-asynq.md) | asynq on Redis for jobs and scheduling | Accepted |
| [0005](0005-storage-garage.md) | Garage object storage, presigned uploads, server-side compression | Accepted |
| [0006](0006-postgres-sqlc-goose.md) | PostgreSQL with sqlc and goose | Accepted |
| [0007](0007-review-power.md) | Review power model: auto-approve, dispute, backer override | Accepted |
| [0008](0008-run-modes.md) | Three run modes driven by Bun scripts | Accepted |
| [0009](0009-visual-direction.md) | Visual direction: Buku Tabungan (passbook) | Accepted |
| [0010](0010-api-conventions.md) | API conventions: errors, idempotency, bodies, pagination | Accepted |
