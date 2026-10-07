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
insert into attachments (id, owner_id, pact_id, kind, declared_mime, declared_bytes, staging_key, created_at)
values ($1, $2, $3, $4, $5, $6, $7, $8)
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

-- name: MarkAttachmentUploaded :one
-- Only the first call wins; a repeat of "complete" finds no row and reads the current one.
update attachments set status = 'uploaded' where id = $1 and status = 'awaiting_upload' returning *;

-- name: ClaimAttachmentForProcessing :one
-- 'processing' is claimable again so a task retried after a crash picks up where it died; asynq
-- runs one attempt of a task at a time and the task id is unique per attachment.
update attachments set status = 'processing' where id = $1 and status in ('uploaded', 'processing') returning *;

-- name: MarkAttachmentReady :exec
update attachments
set status = 'ready', media_key = $2, thumb_key = $3, sniffed_mime = $4, stored_bytes = $5,
    width = $6, height = $7, duration_ms = $8, ready_at = $9
where id = $1;

-- name: RejectAttachment :exec
update attachments set status = 'rejected', reject_reason = $2 where id = $1;

-- name: SumPactAttachmentBytes :one
select coalesce(sum(coalesce(stored_bytes, declared_bytes)), 0)::bigint from attachments
where pact_id = $1 and status <> 'rejected';

-- name: ListStaleUploads :many
-- Slots never completed, or completed and never processed, older than the cutoff.
select * from attachments
where status in ('awaiting_upload', 'uploaded') and created_at < $1
order by created_at limit $2;

-- name: ListOrphanAttachments :many
-- Processed but never attached to a proof, or rejected, older than the cutoff.
select * from attachments
where status in ('ready', 'rejected') and proof_id is null and created_at < $1
order by created_at limit $2;

-- name: ListStuckUploaded :many
-- Completed uploads whose processing task never ran (for example Redis was down at enqueue time).
select * from attachments where status = 'uploaded' and created_at < $1 order by created_at limit $2;

-- name: DeleteAttachment :exec
delete from attachments where id = $1;
