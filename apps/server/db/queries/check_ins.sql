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

-- name: ListReviewQueue :many
select c.*, p.title as pact_title from check_ins c
join pacts p on p.id = c.pact_id
where c.reviewer_id = $1 and c.status = 'submitted'
order by c.review_deadline;

-- name: ListOpenCheckInsForMember :many
select c.*, p.title as pact_title from check_ins c
join pacts p on p.id = c.pact_id
where c.member_id = sqlc.arg(member_id) and p.status = 'active' and not c.is_final
  and c.cutoff_at between sqlc.arg(from_time) and sqlc.arg(to_time)
order by c.cutoff_at;

-- name: CountNonFinalCheckIns :one
select count(*) from check_ins where pact_id = $1 and not is_final;

-- name: CountCheckInsForPact :one
select count(*) from check_ins where pact_id = $1;
