-- name: InsertCheckIns :copyfrom
insert into check_ins (id, pact_id, member_id, reviewer_id, local_date, cutoff_at, submit_deadline)
values ($1, $2, $3, $4, $5, $6, $7);

-- name: GetCheckIn :one
select * from check_ins where id = $1;

-- name: GetCheckInForMember :one
select c.* from check_ins c
join pact_members m on m.pact_id = c.pact_id and m.user_id = sqlc.arg(user_id)
where c.id = sqlc.arg(id);

-- name: GetCheckInForUpdate :one
select * from check_ins where id = $1 for update;

-- Used by settlement: another worker holding the row means it is being handled.
-- name: GetCheckInForUpdateSkipLocked :one
select * from check_ins where id = $1 for update skip locked;

-- Writes every mutable field, guarded by the status the transition started from
-- (optimistic check, SPEC §7). 0 rows = someone else moved it first.
-- name: UpdateCheckInState :execrows
update check_ins
set status = sqlc.arg(status), is_final = sqlc.arg(is_final),
    submitted_at = sqlc.arg(submitted_at), review_deadline = sqlc.arg(review_deadline),
    decided_at = sqlc.arg(decided_at), dispute_deadline = sqlc.arg(dispute_deadline),
    disputed_at = sqlc.arg(disputed_at), resolution_deadline = sqlc.arg(resolution_deadline),
    override_deadline = sqlc.arg(override_deadline), penalty_applied = sqlc.arg(penalty_applied),
    updated_at = now()
where id = sqlc.arg(id) and status = sqlc.arg(expected_status) and is_final = sqlc.arg(expected_final);

-- Every check-in with a passed deadline, oldest deadline first (SPEC §7 steps 1-5).
-- name: ListDueCheckInIDs :many
select id from check_ins
where not is_final and (
     (status = 'open'          and submit_deadline     < sqlc.arg(now)::timestamptz)
  or (status = 'submitted'     and review_deadline     < sqlc.arg(now)::timestamptz)
  or (status = 'auto_approved' and override_deadline   < sqlc.arg(now)::timestamptz)
  or (status = 'rejected'      and dispute_deadline    < sqlc.arg(now)::timestamptz)
  or (status = 'disputed'      and resolution_deadline < sqlc.arg(now)::timestamptz)
)
order by case status
  when 'open' then submit_deadline
  when 'submitted' then review_deadline
  when 'auto_approved' then override_deadline
  when 'rejected' then dispute_deadline
  else resolution_deadline
end
limit sqlc.arg(max_rows);

-- name: ListCheckInsForPact :many
select * from check_ins
where pact_id = sqlc.arg(pact_id) and local_date between sqlc.arg(from_date) and sqlc.arg(to_date)
order by local_date, member_id;

-- Keyset page ordered by (review_deadline, id), soonest first. The proof word count and
-- attachment count come from the latest proof version.
-- name: ListReviewQueuePage :many
select sqlc.embed(c), p.title as pact_title,
       coalesce(lp.word_count, 0)::int as word_count,
       (select count(*) from attachments a where a.proof_id = lp.id)::int as attachment_count
from check_ins c
join pacts p on p.id = c.pact_id
left join lateral (
  select id, word_count from proofs pr where pr.check_in_id = c.id order by version desc limit 1
) lp on true
where c.reviewer_id = sqlc.arg(reviewer_id) and c.status = 'submitted'
  and (sqlc.narg(after_deadline)::timestamptz is null
       or (c.review_deadline, c.id) > (sqlc.narg(after_deadline)::timestamptz, sqlc.narg(after_id)::uuid))
order by c.review_deadline, c.id
limit sqlc.arg(max_rows);

-- name: CountReviewQueue :one
select count(*) from check_ins where reviewer_id = $1 and status = 'submitted';

-- My check-ins for the Today screen: dated today in each active pact's timezone, plus an
-- earlier day that is still open because its grace period has not ended.
-- name: ListTodayCheckIns :many
select sqlc.embed(c), p.title as pact_title,
       coalesce(lp.word_count, 0)::int as word_count,
       (lp.id is not null)::boolean as has_proof
from check_ins c
join pacts p on p.id = c.pact_id
left join lateral (
  select id, word_count from proofs pr where pr.check_in_id = c.id order by version desc limit 1
) lp on true
where c.member_id = sqlc.arg(member_id) and p.status = 'active'
  and (c.local_date = (sqlc.arg(now)::timestamptz at time zone p.timezone)::date
       or (c.status = 'open' and c.local_date < (sqlc.arg(now)::timestamptz at time zone p.timezone)::date
           and c.submit_deadline > sqlc.arg(now)::timestamptz))
order by c.submit_deadline, c.id;

-- My nearest unfinished deadline per pact (open days only).
-- name: NextDeadlines :many
select pact_id, min(submit_deadline)::timestamptz as next_deadline from check_ins
where member_id = sqlc.arg(member_id) and status = 'open' and pact_id = any(sqlc.arg(pact_ids)::uuid[])
group by pact_id;

-- name: CountNonFinalCheckIns :one
select count(*) from check_ins where pact_id = $1 and not is_final;

-- name: CountCheckInsForPact :one
select count(*) from check_ins where pact_id = $1;

-- Reminders (SPEC §9). Open check-ins of active pacts whose cutoff falls in (from_at, to_at]
-- and that were not yet reminded with this kind.
-- name: ListCutoffReminderCandidates :many
select c.* from check_ins c
join pacts p on p.id = c.pact_id
where p.status = 'active' and c.status = 'open'
  and c.cutoff_at > sqlc.arg(from_at)::timestamptz and c.cutoff_at <= sqlc.arg(to_at)::timestamptz
  and not exists (select 1 from reminders_sent r where r.check_in_id = c.id and r.kind = sqlc.arg(kind)::text)
order by c.cutoff_at
limit sqlc.arg(max_rows);

-- Submitted check-ins whose review deadline falls in (from_at, to_at] and whose reviewer
-- was not yet reminded.
-- name: ListReviewReminderCandidates :many
select c.* from check_ins c
join pacts p on p.id = c.pact_id
where p.status = 'active' and c.status = 'submitted'
  and c.review_deadline > sqlc.arg(from_at)::timestamptz and c.review_deadline <= sqlc.arg(to_at)::timestamptz
  and not exists (select 1 from reminders_sent r where r.check_in_id = c.id and r.kind = 'review_deadline_soon')
order by c.review_deadline
limit sqlc.arg(max_rows);

-- 0 rows = this reminder was already sent (possibly by another worker).
-- name: InsertReminderSent :execrows
insert into reminders_sent (check_in_id, kind) values ($1, $2) on conflict do nothing;
