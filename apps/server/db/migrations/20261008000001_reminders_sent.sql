-- Reminder dedupe for the reminders:cutoff job (SPEC §9). The job runs every five minutes
-- and must notify a member about a given check-in once per reminder kind.

-- +goose Up
-- +goose StatementBegin
create table reminders_sent (
  check_in_id uuid not null references check_ins(id) on delete cascade,
  kind        text not null check (kind in ('reminder_cutoff_3h','reminder_cutoff_30m','review_deadline_soon')),
  sent_at     timestamptz not null default now(),
  primary key (check_in_id, kind)
);
-- +goose StatementEnd

-- +goose Down
-- Dev only: production migrations are forward-only (ADR-0006).
-- +goose StatementBegin
drop table if exists reminders_sent;
-- +goose StatementEnd
