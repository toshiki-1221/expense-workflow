package service

import (
	"context"

	"github.com/toshiki-1221/expense-workflow/internal/domain"
)

// categoryRepository は科目一覧を取得するRepositoryの振る舞いを定義する。
type categoryRepository interface {
	ListCategories(ctx context.Context) ([]domain.Category, error)
}

// CategoryService は科目一覧のユースケースを提供する。
type CategoryService struct {
	repository categoryRepository
}

// NewCategoryService は科目Serviceを作成する。
func NewCategoryService(repository categoryRepository) *CategoryService {
	return &CategoryService{repository: repository}
}

// ListCategories は科目一覧を取得する。
func (s *CategoryService) ListCategories(ctx context.Context) ([]domain.Category, error) {
	return s.repository.ListCategories(ctx)
}
