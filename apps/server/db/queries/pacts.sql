-- name: CreatePact :one
insert into pacts (id, title, description, created_by, backer_id, terms, terms_hash, timezone, starts_on, ends_on)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
returning *;

-- name: GetPact :one
select * from pacts where id = $1;

-- Locks the pact row: serialises ledger clamps and counters for one pact (ADR-0002).
-- name: GetPactForUpdate :one
select * from pacts where id = $1 for update;

-- Membership-filtered read (AGENTS.md invariant 8): no row for non-members.
-- name: GetPactForMember :one
select p.* from pacts p
join pact_members m on m.pact_id = p.id and m.user_id = sqlc.arg(user_id)
where p.id = sqlc.arg(pact_id);

-- name: ListPactsForUser :many
select p.* from pacts p
join pact_members m on m.pact_id = p.id
where m.user_id = $1
order by p.created_at desc;

-- Keyset page, newest first. The cursor is the last row's (created_at, id).
-- name: ListPactsForUserPage :many
select p.* from pacts p
join pact_members m on m.pact_id = p.id and m.user_id = sqlc.arg(user_id)
where sqlc.narg(before_at)::timestamptz is null
   or (p.created_at, p.id) < (sqlc.narg(before_at)::timestamptz, sqlc.narg(before_id)::uuid)
order by p.created_at desc, p.id desc
limit sqlc.arg(max_rows);

-- name: CountOpenPactsForUser :one
select count(*) from pacts p
join pact_members m on m.pact_id = p.id
where m.user_id = $1 and p.status not in ('completed', 'cancelled');

-- Terms can change only before both members accepted (SPEC §3).
-- name: UpdatePactTerms :execrows
update pacts
set title = sqlc.arg(title), description = sqlc.arg(description),
    terms = sqlc.arg(terms), terms_hash = sqlc.arg(terms_hash), terms_version = terms_version + 1,
    timezone = sqlc.arg(timezone), starts_on = sqlc.arg(starts_on), ends_on = sqlc.arg(ends_on),
    updated_at = now()
where id = sqlc.arg(id) and status in ('draft', 'proposed');

-- Guarded status change: affects 0 rows when the pact is not in from_status.
-- name: SetPactStatus :execrows
update pacts
set status = sqlc.arg(to_status), updated_at = now(),
    scheduled_at = case when sqlc.arg(to_status)::text = 'scheduled' then now() else scheduled_at end,
    settled_at   = case when sqlc.arg(to_status)::text = 'settling'  then now() else settled_at end,
    completed_at = case when sqlc.arg(to_status)::text = 'completed' then now() else completed_at end
where id = sqlc.arg(id) and status = sqlc.arg(from_status);

-- name: ActivateDuePacts :many
update pacts
set status = 'active', updated_at = now()
where status = 'scheduled'
  and (sqlc.arg(now)::timestamptz at time zone timezone)::date >= starts_on
returning id;

-- Active pacts past their end date whose every check-in is final (SPEC §7 step 7).
-- name: ListSettlablePacts :many
select p.id from pacts p
where p.status = 'active'
  and (sqlc.arg(now)::timestamptz at time zone p.timezone)::date > p.ends_on
  and not exists (select 1 from check_ins c where c.pact_id = p.id and not c.is_final)
order by p.ends_on
limit sqlc.arg(max_rows);

-- name: IncrementOverridesUsed :exec
update pacts set overrides_used = overrides_used + 1, updated_at = now() where id = $1;

-- name: AddPactMember :exec
insert into pact_members (pact_id, user_id, role, line_color)
values ($1, $2, $3, $4);

-- name: ListPactMembers :many
select m.*, u.display_name, u.email from pact_members m
join users u on u.id = m.user_id
where m.pact_id = $1
order by m.role;

-- name: ListMembersForPacts :many
select m.*, u.display_name, u.email from pact_members m
join users u on u.id = m.user_id
where m.pact_id = any(sqlc.arg(pact_ids)::uuid[])
order by m.pact_id, m.role;

-- name: GetPactMember :one
select * from pact_members where pact_id = $1 and user_id = $2;

-- name: ResetAcceptances :exec
update pact_members
set accepted_terms_hash = null, accepted_at = null, signature_name = null
where pact_id = $1;

-- name: AcceptTerms :execrows
update pact_members
set accepted_terms_hash = sqlc.arg(terms_hash), accepted_at = now(), signature_name = sqlc.arg(signature_name)
where pact_id = sqlc.arg(pact_id) and user_id = sqlc.arg(user_id);

-- name: CountAcceptances :one
select count(*) from pact_members where pact_id = sqlc.arg(pact_id) and accepted_terms_hash = sqlc.arg(terms_hash);

-- name: IncrementRestDaysUsed :exec
update pact_members set rest_days_used = rest_days_used + 1 where pact_id = $1 and user_id = $2;

-- name: CreateInvite :exec
insert into pact_invites (token_hash, pact_id, email, expires_at) values ($1, $2, $3, $4);

-- name: GetInvite :one
select * from pact_invites where token_hash = $1;

-- name: UseInvite :execrows
-- Expiry is checked by the service against its injected clock.
update pact_invites set used_at = now() where token_hash = $1 and used_at is null;
