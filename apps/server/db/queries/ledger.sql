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

-- Balances for several pacts at once (list and Today screens).
-- name: PotBalances :many
select pact_id, coalesce(sum(amount), 0)::bigint as balance from ledger_entries
where pact_id = any(sqlc.arg(pact_ids)::uuid[])
group by pact_id;

-- Passbook page, newest first, with the running balance after each line. The window
-- runs over the whole pact ledger before the cursor filter, so every page agrees.
-- The check-in join only adds the day and member a line is about.
-- name: ListLedgerPage :many
select * from (
  select l.id, l.pact_id, l.kind, l.amount, l.check_in_id, l.reverses_entry_id, l.idempotency_key, l.note, l.created_at,
         (sum(l.amount) over (order by l.id))::bigint as balance_after,
         c.local_date as check_in_local_date, c.member_id as check_in_member_id
  from ledger_entries l
  left join check_ins c on c.id = l.check_in_id
  where l.pact_id = sqlc.arg(pact_id)
) page
where sqlc.narg(before_id)::bigint is null or page.id < sqlc.narg(before_id)::bigint
order by page.id desc
limit sqlc.arg(max_rows);

-- name: CreatePayout :exec
insert into payouts (pact_id, amount) values ($1, $2) on conflict (pact_id) do nothing;

-- name: GetPayout :one
select * from payouts where pact_id = $1;

-- name: ListPayoutsForPacts :many
select * from payouts where pact_id = any(sqlc.arg(pact_ids)::uuid[]);

-- name: MarkPayoutPaid :execrows
update payouts set marked_paid_at = now(), marked_paid_note = $2 where pact_id = $1 and marked_paid_at is null;

-- name: ConfirmPayout :execrows
update payouts set confirmed_at = now() where pact_id = $1 and confirmed_at is null;

-- name: InsertNotification :one
insert into notifications (user_id, kind, payload) values ($1, $2, $3) returning id;

-- name: ListNotifications :many
select * from notifications
where user_id = sqlc.arg(user_id)
  and (sqlc.narg(before_id)::bigint is null or id < sqlc.narg(before_id)::bigint)
  and (not sqlc.arg(unread_only)::boolean or read_at is null)
order by id desc
limit sqlc.arg(max_rows);

-- name: CountUnreadNotifications :one
select count(*) from notifications where user_id = $1 and read_at is null;

-- name: MarkNotificationsRead :exec
update notifications set read_at = now() where user_id = sqlc.arg(user_id) and id = any(sqlc.arg(ids)::bigint[]) and read_at is null;

-- name: InsertOutbox :exec
insert into outbox (topic, payload) values ($1, $2);

-- name: FetchPendingOutbox :many
select * from outbox where dispatched_at is null order by id limit $1 for update skip locked;

-- name: MarkOutboxDispatched :exec
-- An invite mail row carries the plaintext invite token until it is relayed; drop it here so
-- only the hash stays in the database.
update outbox set dispatched_at = now(), payload = payload - 'token' where id = any(sqlc.arg(ids)::bigint[]);

-- name: ClaimNotificationEmail :one
-- Claims one notification for emailing and returns what the mail needs. No row: already
-- claimed or sent, or the notification is gone.
with c as (
  update notifications set emailed_at = now()
  where notifications.id = $1 and notifications.emailed_at is null
  returning notifications.id, notifications.user_id, notifications.kind, notifications.payload
)
select c.id, c.kind, c.payload, u.email, u.display_name, u.locale
from c join users u on u.id = c.user_id;

-- name: ReleaseNotificationEmail :exec
update notifications set emailed_at = null where id = any(sqlc.arg(ids)::bigint[]);

-- name: ClaimDigestNotifications :many
-- Claims every not-yet-emailed notification of one kind for a user and pact.
update notifications set emailed_at = now()
where user_id = $1 and kind = $2 and emailed_at is null and payload->>'pact_id' = sqlc.arg(pact_id)::text
returning id;
