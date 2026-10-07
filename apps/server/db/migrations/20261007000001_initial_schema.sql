-- Initial Tepati schema. Design reference: docs/ARCHITECTURE.md §4, rules: docs/SPEC.md.
-- Every foreign key has an index (AGENTS.md conventions).

-- +goose Up
-- +goose StatementBegin
create extension if not exists citext;
create extension if not exists pgcrypto;

create table users (
  id                uuid primary key,
  email             citext not null unique,
  password_hash     text not null,
  display_name      text not null check (length(display_name) between 1 and 80),
  locale            text not null default 'id' check (locale in ('id', 'en')),
  timezone          text not null default 'Asia/Jakarta',
  avatar_key        text,
  email_verified_at timestamptz,
  created_at        timestamptz not null default now()
);

create table pacts (
  id             uuid primary key,
  title          text not null check (length(title) between 1 and 120),
  description    text,
  status         text not null default 'draft'
                 check (status in ('draft','proposed','scheduled','active','settling','completed','cancelled')),
  created_by     uuid not null references users(id),
  backer_id      uuid not null references users(id),
  terms          jsonb not null,
  terms_version  int not null default 1 check (terms_version >= 1),
  terms_hash     text not null,
  timezone       text not null,
  starts_on      date not null,
  ends_on        date not null,
  overrides_used int not null default 0 check (overrides_used >= 0),
  scheduled_at   timestamptz,
  settled_at     timestamptz,
  completed_at   timestamptz,
  created_at     timestamptz not null default now(),
  updated_at     timestamptz not null default now(),
  -- SPEC §3: a pact lasts 1..366 days
  check (ends_on > starts_on and ends_on - starts_on <= 366)
);
create index pacts_created_by_idx on pacts (created_by);
create index pacts_backer_id_idx on pacts (backer_id);
create index pacts_status_idx on pacts (status) where status in ('scheduled', 'active', 'settling');

create table pact_members (
  pact_id             uuid not null references pacts(id) on delete cascade,
  user_id             uuid not null references users(id),
  role                text not null check (role in ('backer', 'doer')),
  line_color          text not null,
  accepted_terms_hash text,
  accepted_at         timestamptz,
  signature_name      text,
  rest_days_used      int not null default 0 check (rest_days_used >= 0),
  primary key (pact_id, user_id)
);
create index pact_members_user_id_idx on pact_members (user_id);
-- exactly one backer per pact (SPEC §2)
create unique index pact_members_one_backer on pact_members (pact_id) where role = 'backer';

create table pact_invites (
  token_hash text primary key,
  pact_id    uuid not null references pacts(id) on delete cascade,
  email      citext,
  expires_at timestamptz not null,
  used_at    timestamptz,
  created_at timestamptz not null default now()
);
create index pact_invites_pact_id_idx on pact_invites (pact_id);

create table check_ins (
  id                  uuid primary key,
  pact_id             uuid not null references pacts(id) on delete cascade,
  member_id           uuid not null references users(id),
  reviewer_id         uuid not null references users(id),
  local_date          date not null,
  status              text not null default 'open'
                      check (status in ('open','submitted','approved','auto_approved','rejected','disputed','missed','rest')),
  is_final            boolean not null default false,
  cutoff_at           timestamptz not null,
  submit_deadline     timestamptz not null,
  submitted_at        timestamptz,
  review_deadline     timestamptz,
  decided_at          timestamptz,
  dispute_deadline    timestamptz,
  disputed_at         timestamptz,
  resolution_deadline timestamptz,
  override_deadline   timestamptz,
  penalty_applied     boolean not null default false,
  created_at          timestamptz not null default now(),
  updated_at          timestamptz not null default now(),
  unique (pact_id, member_id, local_date),
  -- SPEC §2: nobody reviews their own check-in
  check (member_id <> reviewer_id),
  check (submit_deadline >= cutoff_at)
);
create index check_ins_member_id_idx on check_ins (member_id);
create index check_ins_reviewer_queue_idx on check_ins (reviewer_id, review_deadline) where status = 'submitted';
-- settlement sweeps (SPEC §7) only look at non-final rows
create index check_ins_sweep_open_idx on check_ins (submit_deadline) where status = 'open';
create index check_ins_sweep_submitted_idx on check_ins (review_deadline) where status = 'submitted';
create index check_ins_sweep_auto_idx on check_ins (override_deadline) where status = 'auto_approved' and not is_final;
create index check_ins_sweep_rejected_idx on check_ins (dispute_deadline) where status = 'rejected' and not is_final;
create index check_ins_sweep_disputed_idx on check_ins (resolution_deadline) where status = 'disputed';

create table proofs (
  id          uuid primary key,
  check_in_id uuid not null references check_ins(id) on delete cascade,
  version     int not null check (version >= 1),
  body_doc    jsonb not null,
  body_text   text not null,
  word_count  int not null check (word_count >= 0),
  links       jsonb not null default '[]',
  created_at  timestamptz not null default now(),
  unique (check_in_id, version)
);

create table attachments (
  id             uuid primary key,
  owner_id       uuid not null references users(id),
  pact_id        uuid not null references pacts(id) on delete cascade,
  proof_id       uuid references proofs(id) on delete set null,
  kind           text not null check (kind in ('image', 'video', 'file')),
  status         text not null default 'awaiting_upload'
                 check (status in ('awaiting_upload','uploaded','processing','ready','rejected')),
  staging_key    text,
  media_key      text,
  thumb_key      text,
  declared_mime  text,
  sniffed_mime   text,
  declared_bytes bigint check (declared_bytes > 0),
  stored_bytes   bigint check (stored_bytes >= 0),
  width          int,
  height         int,
  duration_ms    int,
  reject_reason  text,
  created_at     timestamptz not null default now(),
  ready_at       timestamptz
);
create index attachments_owner_id_idx on attachments (owner_id);
create index attachments_pact_id_idx on attachments (pact_id);
create index attachments_proof_id_idx on attachments (proof_id);
create index attachments_gc_idx on attachments (created_at) where status in ('awaiting_upload', 'uploaded');

-- Audit of every review action; backer power must be visible (SPEC §2, ADR-0007).
create table decisions (
  id          bigserial primary key,
  check_in_id uuid not null references check_ins(id) on delete cascade,
  actor_id    uuid references users(id), -- null = system
  action      text not null check (action in
              ('submit','approve','reject','auto_approve','override','dispute','uphold','dismiss','dispute_expired','rest','missed','finalize')),
  reason      text,
  created_at  timestamptz not null default now(),
  check (action not in ('reject', 'override', 'dismiss') or length(coalesce(reason, '')) >= 10)
);
create index decisions_check_in_id_idx on decisions (check_in_id, id);
create index decisions_actor_id_idx on decisions (actor_id);

-- Append-only coin ledger (SPEC §6, ADR-0002). Balance = SUM(amount).
create table ledger_entries (
  id                bigserial primary key,
  pact_id           uuid not null references pacts(id),
  kind              text not null check (kind in ('pot_initial','doer_miss','backer_miss','reversal','payout')),
  amount            bigint not null,
  check_in_id       uuid references check_ins(id),
  reverses_entry_id bigint references ledger_entries(id),
  idempotency_key   text not null unique,
  note              text,
  created_at        timestamptz not null default now(),
  check (kind <> 'reversal' or reverses_entry_id is not null)
);
create index ledger_entries_pact_idx on ledger_entries (pact_id, id);
create index ledger_entries_check_in_id_idx on ledger_entries (check_in_id);
create index ledger_entries_reverses_idx on ledger_entries (reverses_entry_id);

create function ledger_entries_forbid_mutation() returns trigger language plpgsql as $$
begin
  raise exception 'ledger_entries is append-only (ADR-0002)' using errcode = 'restrict_violation';
end
$$;
create trigger ledger_entries_no_update_delete
  before update or delete on ledger_entries
  for each row execute function ledger_entries_forbid_mutation();

create table payouts (
  pact_id          uuid primary key references pacts(id),
  amount           bigint not null check (amount >= 0),
  marked_paid_at   timestamptz,
  marked_paid_note text,
  confirmed_at     timestamptz,
  created_at       timestamptz not null default now()
);

create table notifications (
  id         bigserial primary key,
  user_id    uuid not null references users(id) on delete cascade,
  kind       text not null,
  payload    jsonb not null,
  read_at    timestamptz,
  created_at timestamptz not null default now()
);
create index notifications_user_unread_idx on notifications (user_id, id desc) where read_at is null;
create index notifications_user_id_idx on notifications (user_id, id desc);

-- Transactional outbox: side effects that must be emitted exactly when a tx commits.
create table outbox (
  id            bigserial primary key,
  topic         text not null,
  payload       jsonb not null,
  created_at    timestamptz not null default now(),
  dispatched_at timestamptz
);
create index outbox_pending_idx on outbox (id) where dispatched_at is null;
-- +goose StatementEnd

-- +goose Down
-- Dev only: production migrations are forward-only (ADR-0006).
-- +goose StatementBegin
drop table if exists outbox, notifications, payouts;
drop trigger if exists ledger_entries_no_update_delete on ledger_entries;
drop function if exists ledger_entries_forbid_mutation();
drop table if exists ledger_entries, decisions, attachments, proofs, check_ins, pact_invites, pact_members, pacts, users;
-- +goose StatementEnd
