package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/toshiki-1221/expense-workflow/internal/auth"
	"github.com/toshiki-1221/expense-workflow/internal/domain"
)

type fakeUsers struct {
	byEmail map[string]*domain.User
	byID    map[int64]*domain.User
	err     error
}

func (f *fakeUsers) FindByEmail(_ context.Context, email string) (*domain.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	if u, ok := f.byEmail[email]; ok {
		return u, nil
	}
	return nil, domain.ErrNotFound
}

func (f *fakeUsers) FindByID(_ context.Context, id int64) (*domain.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	if u, ok := f.byID[id]; ok {
		return u, nil
	}
	return nil, domain.ErrNotFound
}

type fakeTokens struct{}

func (fakeTokens) Issue(id int64) (string, error) { return fmt.Sprintf("token-%d", id), nil }

func TestAuthService_Login(t *testing.T) {
	hash, err := auth.HashPassword("correct-password")
	if err != nil {
		t.Fatal(err)
	}
	yamada := &domain.User{ID: 1, Email: "yamada@example.com", PasswordHash: hash}
	svc := NewAuthService(&fakeUsers{byEmail: map[string]*domain.User{yamada.Email: yamada}}, fakeTokens{})

	tests := []struct {
		name, email, password string
		wantErr               error
	}{
		{"成功", "yamada@example.com", "correct-password", nil},
		{"大文字・前後の空白は正規化して照合", " Yamada@Example.com ", "correct-password", nil},
		{"パスワード違い", "yamada@example.com", "wrong", domain.ErrInvalidCredentials},
		{"存在しないメールも同じエラー", "nobody@example.com", "correct-password", domain.ErrInvalidCredentials},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := svc.Login(context.Background(), tt.email, tt.password)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v; want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && (res.Token != "token-1" || res.User.ID != 1) {
				t.Fatalf("res = %+v; want token-1 / user 1", res)
			}
		})
	}

	t.Run("DBエラーは ErrInvalidCredentials にしない", func(t *testing.T) {
		dbErr := errors.New("connection refused")
		svc := NewAuthService(&fakeUsers{err: dbErr}, fakeTokens{})
		if _, err := svc.Login(context.Background(), "yamada@example.com", "x"); !errors.Is(err, dbErr) {
			t.Fatalf("err = %v; want %v", err, dbErr)
		}
	})
}

func TestAuthService_CurrentUser(t *testing.T) {
	svc := NewAuthService(&fakeUsers{byID: map[int64]*domain.User{1: {ID: 1}}}, fakeTokens{})

	if u, err := svc.CurrentUser(context.Background(), 1); err != nil || u.ID != 1 {
		t.Fatalf("CurrentUser(1) = %+v, %v; want user 1", u, err)
	}
	// トークンは有効でもユーザーが消えていたら未認証
	if _, err := svc.CurrentUser(context.Background(), 999); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("err = %v; want ErrUnauthorized", err)
	}
}
