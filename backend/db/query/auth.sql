-- name: GetUserByEmail :one
SELECT id, email, password_hash, name, role, approver_id, created_at, updated_at
FROM users
WHERE email = $1;

-- name: GetMe :one
SELECT
    u.id,
    u.email,
    u.name,
    u.role,
    u.approver_id,
    EXISTS (SELECT 1 FROM users direct_reports WHERE direct_reports.approver_id = u.id) AS is_approver
FROM users u
WHERE u.id = $1;
