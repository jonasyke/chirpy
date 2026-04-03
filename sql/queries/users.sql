-- name: CreateUser :one
INSERT INTO users (id, created_at, updated_at, email, hashed_password)
VALUES (
    gen_random_uuid(),
    now(),
    now(),
    $1,
    $2

)
RETURNING *;

-- name: ResetUser :exec
DELETE FROM users;

-- name: LoginLookup :one
SELECT *
FROM users
WHERE email = $1
LIMIT 1;

-- name: UserIDLookup :one
SELECT *
FROM users
WHERE id = $1
LIMIT 1;

-- name: UpdateUser :one
UPDATE users
SET email = $2, hashed_password = $3, updated_at = NOW()
where id = $1
RETURNING *;