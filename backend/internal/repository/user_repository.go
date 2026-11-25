package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/toshiki-1221/expense-workflow/internal/db"
	"github.com/toshiki-1221/expense-workflow/internal/domain"
)

// UserRepository はsqlcを使ってユーザーデータを取得する。
type UserRepository struct {
	queries db.Querier
}

// NewUserRepository はユーザーRepositoryを作成する。
func NewUserRepository(queries db.Querier) *UserRepository {
	return &UserRepository{queries: queries}
}

// FindByEmail はメールアドレスでユーザーを取得する（パスワードハッシュ付き）。
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	row, err := r.queries.GetUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return &domain.User{
		ID:           row.ID,
		Email:        row.Email,
		Name:         row.Name,
		PasswordHash: row.PasswordHash,
		Role:         domain.Role(row.Role),
		ApproverID:   row.ApproverID,
		IsApprover:   row.IsApprover,
	}, nil
}

// FindByID はIDでユーザーを取得する（パスワードハッシュなし）。
func (r *UserRepository) FindByID(ctx context.Context, id int64) (*domain.User, error) {
	row, err := r.queries.GetMe(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &domain.User{
		ID:         row.ID,
		Email:      row.Email,
		Name:       row.Name,
		Role:       domain.Role(row.Role),
		ApproverID: row.ApproverID,
		IsApprover: row.IsApprover,
	}, nil
}
