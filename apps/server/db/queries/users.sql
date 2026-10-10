-- name: CreateUser :one
insert into users (id, email, password_hash, display_name, locale, timezone)
values ($1, $2, $3, $4, $5, $6)
returning *;

-- name: GetUser :one
select * from users where id = $1;

-- name: GetUserByEmail :one
select * from users where email = $1;

-- name: UpdateUserProfile :one
-- Only the columns that are sent change (null = leave as is); email_off is replaced as a whole.
update users set
  display_name = coalesce(sqlc.narg(display_name)::text, display_name),
  locale       = coalesce(sqlc.narg(locale)::text, locale),
  timezone     = coalesce(sqlc.narg(timezone)::text, timezone),
  email_off    = coalesce(sqlc.narg(email_off)::text[], email_off)
where id = sqlc.arg(id)
returning *;

-- name: UpdateUserPassword :exec
update users set password_hash = $2 where id = $1;
