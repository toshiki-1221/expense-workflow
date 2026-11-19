-- name: ListMyExpenseReports :many
SELECT id, user_id, approver_id, title, status, submitted_at, created_at, updated_at
FROM expense_reports
WHERE user_id = sqlc.arg(user_id)
  AND (sqlc.narg(status)::varchar IS NULL OR status = sqlc.narg(status)::varchar)
ORDER BY created_at DESC, id DESC;

-- name: CreateExpenseReport :one
INSERT INTO expense_reports (user_id, title, status)
VALUES (sqlc.arg(user_id), sqlc.arg(title), 'draft')
RETURNING id, user_id, approver_id, title, status, submitted_at, created_at, updated_at;

-- name: GetExpenseReport :one
SELECT id, user_id, approver_id, title, status, submitted_at, created_at, updated_at
FROM expense_reports
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id);

-- name: ListExpenseReportItems :many
SELECT
    ei.id,
    ei.report_id,
    ei.category_id,
    ec.name AS category_name,
    ei.used_on,
    ei.amount,
    ei.description,
    ei.created_at,
    ei.updated_at
FROM expense_items ei
JOIN expense_categories ec ON ec.id = ei.category_id
WHERE ei.report_id = sqlc.arg(report_id)
ORDER BY ei.used_on, ei.id;

-- name: UpdateExpenseReport :one
UPDATE expense_reports
SET title = sqlc.arg(title), updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id)
  AND user_id = sqlc.arg(user_id)
  AND status IN ('draft', 'rejected')
RETURNING id, user_id, approver_id, title, status, submitted_at, created_at, updated_at;

-- name: DeleteDraftExpenseReport :execrows
DELETE FROM expense_reports
WHERE id = sqlc.arg(id)
  AND user_id = sqlc.arg(user_id)
  AND status = 'draft';

-- name: CreateExpenseItem :one
INSERT INTO expense_items (report_id, category_id, used_on, amount, description)
SELECT sqlc.arg(report_id), sqlc.arg(category_id), sqlc.arg(used_on), sqlc.arg(amount), sqlc.arg(description)
WHERE EXISTS (
    SELECT 1 FROM expense_reports
    WHERE id = sqlc.arg(report_id)
      AND user_id = sqlc.arg(user_id)
      AND status IN ('draft', 'rejected')
)
RETURNING id, report_id, category_id, used_on, amount, description, created_at, updated_at;

-- name: UpdateExpenseItem :execrows
UPDATE expense_items ei
SET category_id = sqlc.arg(category_id),
    used_on = sqlc.arg(used_on),
    amount = sqlc.arg(amount),
    description = sqlc.arg(description),
    updated_at = CURRENT_TIMESTAMP
FROM expense_reports er
WHERE ei.id = sqlc.arg(id)
  AND ei.report_id = er.id
  AND er.id = sqlc.arg(report_id)
  AND er.user_id = sqlc.arg(user_id)
  AND er.status IN ('draft', 'rejected');

-- name: DeleteExpenseItem :execrows
DELETE FROM expense_items ei
USING expense_reports er
WHERE ei.id = sqlc.arg(id)
  AND ei.report_id = er.id
  AND er.id = sqlc.arg(report_id)
  AND er.user_id = sqlc.arg(user_id)
  AND er.status IN ('draft', 'rejected');

-- name: SubmitExpenseReport :one
WITH target AS (
    SELECT er.id, er.status AS from_status
    FROM expense_reports er
    WHERE er.id = sqlc.arg(report_id)
      AND er.user_id = sqlc.arg(user_id)
      AND er.status IN ('draft', 'rejected')
    FOR UPDATE
), updated AS (
    UPDATE expense_reports er
    SET status = 'submitted',
        approver_id = u.approver_id,
        submitted_at = CURRENT_TIMESTAMP,
        updated_at = CURRENT_TIMESTAMP
    FROM users u, target t
    WHERE er.id = t.id
      AND u.id = er.user_id
      AND u.approver_id IS NOT NULL
    RETURNING er.id, er.user_id, er.approver_id, er.title, er.status,
              er.submitted_at, er.created_at, er.updated_at, t.from_status
), logged AS (
    INSERT INTO approval_logs (report_id, actor_id, action, from_status, to_status)
    SELECT updated.id, updated.user_id, 'submit', updated.from_status, 'submitted' FROM updated
)
SELECT updated.id, updated.user_id, updated.approver_id, updated.title, updated.status,
       updated.submitted_at, updated.created_at, updated.updated_at
FROM updated;

-- name: ListPendingApprovals :many
SELECT er.id, er.user_id, submitter.name AS user_name, er.approver_id,
       er.title, er.status, er.submitted_at, er.created_at, er.updated_at
FROM expense_reports er
JOIN users submitter ON submitter.id = er.user_id
WHERE er.approver_id = sqlc.arg(approver_id)
  AND er.status = 'submitted'
ORDER BY er.submitted_at, er.id;

-- name: ApproveExpenseReport :one
WITH updated AS (
    UPDATE expense_reports er
    SET status = 'approved', updated_at = CURRENT_TIMESTAMP
    WHERE er.id = sqlc.arg(report_id)
      AND er.approver_id = sqlc.arg(actor_id)
      AND er.status = 'submitted'
    RETURNING er.id, er.user_id, er.approver_id, er.title, er.status, er.submitted_at, er.created_at, er.updated_at
), logged AS (
    INSERT INTO approval_logs (report_id, actor_id, action, from_status, to_status)
    SELECT updated.id, sqlc.arg(actor_id), 'approve', 'submitted', 'approved' FROM updated
)
SELECT * FROM updated;

-- name: RejectExpenseReport :one
WITH updated AS (
    UPDATE expense_reports er
    SET status = 'rejected', updated_at = CURRENT_TIMESTAMP
    WHERE er.id = sqlc.arg(report_id)
      AND er.approver_id = sqlc.arg(actor_id)
      AND er.status = 'submitted'
      AND BTRIM(sqlc.arg(comment)::text) <> ''
    RETURNING er.id, er.user_id, er.approver_id, er.title, er.status, er.submitted_at, er.created_at, er.updated_at
), logged AS (
    INSERT INTO approval_logs (report_id, actor_id, action, from_status, to_status, comment)
    SELECT updated.id, sqlc.arg(actor_id), 'reject', 'submitted', 'rejected', sqlc.arg(comment) FROM updated
)
SELECT * FROM updated;

-- name: PayExpenseReport :one
WITH updated AS (
    UPDATE expense_reports er
    SET status = 'paid', updated_at = CURRENT_TIMESTAMP
    WHERE er.id = sqlc.arg(report_id)
      AND er.status = 'approved'
      AND EXISTS (
          SELECT 1 FROM users actor
          WHERE actor.id = sqlc.arg(actor_id)
            AND actor.role = 'accounting'
      )
    RETURNING er.id, er.user_id, er.approver_id, er.title, er.status, er.submitted_at, er.created_at, er.updated_at
), logged AS (
    INSERT INTO approval_logs (report_id, actor_id, action, from_status, to_status)
    SELECT updated.id, sqlc.arg(actor_id), 'pay', 'approved', 'paid' FROM updated
)
SELECT * FROM updated;

-- name: ListExpenseReportApprovalLogs :many
SELECT id, report_id, actor_id, action, from_status, to_status, comment, created_at
FROM approval_logs
WHERE report_id = sqlc.arg(report_id)
ORDER BY created_at, id;
