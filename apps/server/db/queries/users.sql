-- name: CreateUser :one
insert into users (id, email, password_hash, display_name, locale, timezone)
values ($1, $2, $3, $4, $5, $6)
returning *;

-- name: GetUser :one
select * from users where id = $1;

-- name: GetUserByEmail :one
select * from users where email = $1;
