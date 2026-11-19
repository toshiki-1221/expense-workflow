-- name: ListAccountingExpenseReports :many
SELECT er.id, er.user_id, submitter.name AS user_name,
       er.approver_id, er.title, er.status, er.submitted_at,
       er.created_at, er.updated_at
FROM expense_reports er
JOIN users submitter ON submitter.id = er.user_id
WHERE sqlc.narg(status)::varchar IS NULL OR er.status = sqlc.narg(status)::varchar
ORDER BY er.created_at DESC, er.id DESC
LIMIT sqlc.arg(per_page)::int
OFFSET GREATEST(sqlc.arg(page)::int - 1, 0) * sqlc.arg(per_page)::int;

-- name: CountAccountingExpenseReports :one
SELECT COUNT(*)
FROM expense_reports er
WHERE sqlc.narg(status)::varchar IS NULL OR er.status = sqlc.narg(status)::varchar;
