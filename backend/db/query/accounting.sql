-- name: ListAccountingExpenseReports :many
-- 経理の一覧。下書きは申請者の作業中なので出さない（一度でも申請したものだけ）。
-- 新しく申請された順。オフセット方式でページングする（page は 1 始まり）。
SELECT
    er.id,
    er.user_id,
    submitter.name AS user_name,
    er.title,
    er.status,
    er.submitted_at,
    er.created_at,
    er.updated_at,
    COALESCE(SUM(ei.amount), 0)::bigint AS total_amount,
    COUNT(ei.id)::bigint AS item_count
FROM expense_reports er
JOIN users submitter ON submitter.id = er.user_id
LEFT JOIN expense_items ei ON ei.report_id = er.id
WHERE er.status <> 'draft'
  AND (sqlc.narg(status)::varchar IS NULL OR er.status = sqlc.narg(status)::varchar)
GROUP BY er.id, submitter.name
ORDER BY er.submitted_at DESC, er.id DESC
LIMIT sqlc.arg(per_page)::int
OFFSET (sqlc.arg(page)::int - 1) * sqlc.arg(per_page)::int;

-- name: CountAccountingExpenseReports :one
-- 一覧と同じ条件の総件数（ページ数を出すため）
SELECT COUNT(*)
FROM expense_reports er
WHERE er.status <> 'draft'
  AND (sqlc.narg(status)::varchar IS NULL OR er.status = sqlc.narg(status)::varchar);
