-- PLAN 9.4: a person can switch off some kinds of email (SPEC §9). The list holds what is OFF, so
-- a kind added later is on for everyone by default. Which kinds may be listed is decided in
-- internal/domain/emailprefs.go, not here.

-- +goose Up
-- +goose StatementBegin
alter table users add column email_off text[] not null default '{}';
-- +goose StatementEnd

-- +goose Down
-- Dev only: production migrations are forward-only (ADR-0006).
-- +goose StatementBegin
alter table users drop column if exists email_off;
-- +goose StatementEnd
