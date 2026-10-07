# ADR-0006: PostgreSQL with sqlc and goose

Status: Accepted (2026-10-07)

## Decision
- **Database:** PostgreSQL 18, using the official Docker image.
- **Queries:** written as plain SQL and compiled with **sqlc** using the pgx/v5 driver.
- **Migrations:** **goose**, as timestamped SQL files.
- **No ORM.**

## Consequences
- Agents write SQL explicitly, so money and time logic stays visible and easy to review.
- Production migrations only move forward. Every migration still needs a `-- +goose Down` section, used in dev only.
- The first migration enables the `citext` and `pgcrypto` extensions.
