package repository

import (
	"context"
	"fmt"

	"github.com/toshiki-1221/expense-workflow/internal/db"
	"github.com/toshiki-1221/expense-workflow/internal/domain"
)

// CategoryRepository はsqlcを使って科目データを取得する。
type CategoryRepository struct {
	queries db.Querier
}

// NewCategoryRepository は科目Repositoryを作成する。
func NewCategoryRepository(queries db.Querier) *CategoryRepository {
	return &CategoryRepository{queries: queries}
}

// ListCategories は表示順に科目一覧を取得する。
func (r *CategoryRepository) ListCategories(ctx context.Context) ([]domain.Category, error) {
	rows, err := r.queries.ListExpenseCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("list expense categories: %w", err)
	}

	categories := make([]domain.Category, 0, len(rows))
	for _, row := range rows {
		categories = append(categories, domain.Category{
			ID:        row.ID,
			Name:      row.Name,
			SortOrder: row.SortOrder,
		})
	}
	return categories, nil
}
