-- name: ListExpenseCategories :many
SELECT id, name, sort_order
FROM expense_categories
ORDER BY sort_order, id;
