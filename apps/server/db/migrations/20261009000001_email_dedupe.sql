-- Email dedupe for PLAN 3.2. An email task can run twice (asynq retries), so the worker
-- claims the row by setting emailed_at before it sends and clears it again if the send fails.

-- +goose Up
-- +goose StatementBegin
alter table notifications add column emailed_at timestamptz;
alter table pact_invites  add column emailed_at timestamptz;
-- +goose StatementEnd

-- +goose Down
-- Dev only: production migrations are forward-only (ADR-0006).
-- +goose StatementBegin
alter table notifications drop column if exists emailed_at;
alter table pact_invites  drop column if exists emailed_at;
-- +goose StatementEnd
