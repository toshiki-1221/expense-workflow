package service

import (
	"context"
	"errors"
	"strings"

	"github.com/toshiki-1221/expense-workflow/internal/auth"
	"github.com/toshiki-1221/expense-workflow/internal/domain"
)

// userRepository はユーザーを取得するRepositoryの振る舞いを定義する。
type userRepository interface {
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	FindByID(ctx context.Context, id int64) (*domain.User, error)
}

// tokenIssuer はトークンを発行する振る舞いを定義する。
type tokenIssuer interface {
	Issue(userID int64) (string, error)
}

// AuthService はログインと現在のユーザー取得のユースケースを提供する。
type AuthService struct {
	users  userRepository
	tokens tokenIssuer
}

// NewAuthService は認証Serviceを作成する。
func NewAuthService(users userRepository, tokens tokenIssuer) *AuthService {
	return &AuthService{users: users, tokens: tokens}
}

// LoginResult はログイン成功時の結果を表す。
type LoginResult struct {
	Token string
	User  *domain.User
}

// Login はメールアドレスとパスワードを照合し、トークンを発行する。
// 失敗理由（メールが無い／パスワード違い）は区別せず ErrInvalidCredentials を返す。
func (s *AuthService) Login(ctx context.Context, email, password string) (*LoginResult, error) {
	email = strings.ToLower(strings.TrimSpace(email)) // 保存時と同じ形に正規化

	u, err := s.users.FindByEmail(ctx, email)
	if errors.Is(err, domain.ErrNotFound) {
		auth.CheckDummy(password) // 応答時間をそろえる
		return nil, domain.ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if !auth.CheckPassword(u.PasswordHash, password) {
		return nil, domain.ErrInvalidCredentials
	}

	token, err := s.tokens.Issue(u.ID)
	if err != nil {
		return nil, err
	}
	return &LoginResult{Token: token, User: u}, nil
}

// CurrentUser はトークンのユーザーIDから最新のユーザー情報を取得する。
// トークンが有効でもユーザーが削除済みなら未認証扱いにする。
func (s *AuthService) CurrentUser(ctx context.Context, userID int64) (*domain.User, error) {
	u, err := s.users.FindByID(ctx, userID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, domain.ErrUnauthorized
	}
	return u, err
}
