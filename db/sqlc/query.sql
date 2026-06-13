-- name: CreateUser :execresult
INSERT INTO users (name, dob)
VALUES (?, ?);

-- name: GetUser :one
SELECT id, name, dob, created_at, updated_at
FROM users
WHERE id = ?
LIMIT 1;

-- name: ListUsers :many
SELECT id, name, dob, created_at, updated_at
FROM users
ORDER BY id ASC
LIMIT ? OFFSET ?;

-- name: CountUsers :one
SELECT COUNT(*) AS total
FROM users;

-- name: UpdateUser :exec
UPDATE users
SET name = ?, dob = ?
WHERE id = ?;

-- name: DeleteUser :exec
DELETE FROM users
WHERE id = ?;
