-- name: GetUserByEmail :one
SELECT
    u.id,
    u.email,
    u.password_hash,
    u.name,
    u.role,
    u.approver_id,
    EXISTS (SELECT 1 FROM users direct_reports WHERE direct_reports.approver_id = u.id) AS is_approver
FROM users u
WHERE u.email = $1;

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
