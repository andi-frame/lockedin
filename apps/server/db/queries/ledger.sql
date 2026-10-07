-- Idempotent insert: returns no row (pgx.ErrNoRows) when the key already exists (SPEC §6 L3).
-- name: InsertLedgerEntry :one
insert into ledger_entries (pact_id, kind, amount, check_in_id, reverses_entry_id, idempotency_key, note)
values ($1, $2, $3, $4, $5, $6, $7)
on conflict (idempotency_key) do nothing
returning *;

-- name: GetLedgerEntryByKey :one
select * from ledger_entries where idempotency_key = $1;

-- Pot balance is always the sum of the ledger (SPEC §6 L1).
-- name: PotBalance :one
select coalesce(sum(amount), 0)::bigint as balance from ledger_entries where pact_id = $1;

-- Passbook page, newest first, with the running balance after each line.
-- name: ListLedgerPage :many
select * from (
  select l.*, (sum(l.amount) over (order by l.id))::bigint as balance_after
  from ledger_entries l
  where l.pact_id = sqlc.arg(pact_id)
) page
where sqlc.narg(before_id)::bigint is null or page.id < sqlc.narg(before_id)::bigint
order by page.id desc
limit sqlc.arg(max_rows);

-- name: CreatePayout :exec
insert into payouts (pact_id, amount) values ($1, $2) on conflict (pact_id) do nothing;

-- name: GetPayout :one
select * from payouts where pact_id = $1;

-- name: MarkPayoutPaid :execrows
update payouts set marked_paid_at = now(), marked_paid_note = $2 where pact_id = $1 and marked_paid_at is null;

-- name: ConfirmPayout :execrows
update payouts set confirmed_at = now() where pact_id = $1 and confirmed_at is null;

-- name: InsertNotification :exec
insert into notifications (user_id, kind, payload) values ($1, $2, $3);

-- name: ListNotifications :many
select * from notifications
where user_id = sqlc.arg(user_id) and (sqlc.narg(before_id)::bigint is null or id < sqlc.narg(before_id)::bigint)
order by id desc
limit sqlc.arg(max_rows);

-- name: MarkNotificationsRead :exec
update notifications set read_at = now() where user_id = sqlc.arg(user_id) and id = any(sqlc.arg(ids)::bigint[]) and read_at is null;

-- name: InsertOutbox :exec
insert into outbox (topic, payload) values ($1, $2);

-- name: FetchPendingOutbox :many
select * from outbox where dispatched_at is null order by id limit $1 for update skip locked;

-- name: MarkOutboxDispatched :exec
update outbox set dispatched_at = now() where id = any(sqlc.arg(ids)::bigint[]);
