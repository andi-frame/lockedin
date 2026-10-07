-- name: InsertProof :one
insert into proofs (id, check_in_id, version, body_doc, body_text, word_count, links)
values ($1, $2,
        (select coalesce(max(version), 0) + 1 from proofs where check_in_id = $2),
        $3, $4, $5, $6)
returning *;

-- name: GetLatestProof :one
select * from proofs where check_in_id = $1 order by version desc limit 1;

-- name: ListProofs :many
select * from proofs where check_in_id = $1 order by version desc;

-- name: CreateAttachment :one
insert into attachments (id, owner_id, pact_id, kind, declared_mime, declared_bytes, staging_key)
values ($1, $2, $3, $4, $5, $6, $7)
returning *;

-- name: GetAttachment :one
select * from attachments where id = $1;

-- name: AttachToProof :execrows
update attachments set proof_id = sqlc.arg(proof_id)
where id = any(sqlc.arg(ids)::uuid[]) and owner_id = sqlc.arg(owner_id) and pact_id = sqlc.arg(pact_id);

-- name: ListAttachmentsForProof :many
select * from attachments where proof_id = $1 order by created_at;

-- name: CountAttachmentsByStatus :many
select status, count(*)::int as n from attachments
where id = any(sqlc.arg(ids)::uuid[]) and owner_id = sqlc.arg(owner_id) and pact_id = sqlc.arg(pact_id)
group by status;

-- name: InsertDecision :exec
insert into decisions (check_in_id, actor_id, action, reason) values ($1, $2, $3, $4);

-- name: ListDecisions :many
select d.*, u.display_name as actor_name from decisions d
left join users u on u.id = d.actor_id
where d.check_in_id = $1
order by d.id;
