package handler

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/toshiki-1221/expense-workflow/internal/domain"
	"github.com/toshiki-1221/expense-workflow/internal/handler/response"
	"github.com/toshiki-1221/expense-workflow/internal/middleware"
	"github.com/toshiki-1221/expense-workflow/internal/service"
)

// authService はログインと現在のユーザー取得を提供するServiceの振る舞いを定義する。
type authService interface {
	Login(ctx context.Context, email, password string) (*service.LoginResult, error)
	CurrentUser(ctx context.Context, userID int64) (*domain.User, error)
}

// AuthHandler はログインと /me のHTTP処理を担当する。
type AuthHandler struct {
	service authService
}

// NewAuthHandler は認証Handlerを作成する。
func NewAuthHandler(service authService) *AuthHandler {
	return &AuthHandler{service: service}
}

// loginRequest はログインAPIのリクエストボディを表す。
type loginRequest struct {
	Email string `json:"email" binding:"required,max=255,email"`
	// max は文字数。bcrypt の 72 バイト制限は auth.CheckPassword がバイト数で守る。
	Password string `json:"password" binding:"required,max=72"`
}

// userResponse はユーザー情報のレスポンス項目を表す。password_hash は絶対に載せない。
type userResponse struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Email      string `json:"email"`
	Role       string `json:"role"`
	IsApprover bool   `json:"is_approver"`
}

// loginResponse はログインAPIのレスポンスを表す。
type loginResponse struct {
	Token string       `json:"token"`
	User  userResponse `json:"user"`
}

// toUserResponse はドメインのユーザーをレスポンス形式に変換する。
func toUserResponse(u *domain.User) userResponse {
	return userResponse{
		ID:         u.ID,
		Name:       u.Name,
		Email:      u.Email,
		Role:       string(u.Role),
		IsApprover: u.IsApprover,
	}
}

// Login はメールアドレスとパスワードでログインし、JWTを返す。
// POST /api/v1/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "メールアドレスとパスワードを正しく入力してください")
		return
	}

	res, err := h.service.Login(c.Request.Context(), req.Email, req.Password)
	if errors.Is(err, domain.ErrInvalidCredentials) {
		// 総当たり検知のため失敗は記録する。パスワードは絶対にログに出さない
		log.Printf("login failed: email=%q ip=%s", req.Email, c.ClientIP())
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "メールアドレスまたはパスワードが正しくありません")
		return
	}
	if err != nil {
		log.Printf("login failed: %v", err)
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "internal server error")
		return
	}

	c.JSON(http.StatusOK, loginResponse{Token: res.Token, User: toUserResponse(res.User)})
}

// Me はログイン中のユーザー情報を返す。
// GET /api/v1/me
func (h *AuthHandler) Me(c *gin.Context) {
	userID, ok := middleware.UserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "ログインしてください")
		return
	}

	u, err := h.service.CurrentUser(c.Request.Context(), userID)
	if errors.Is(err, domain.ErrUnauthorized) {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "ログインしてください")
		return
	}
	if err != nil {
		log.Printf("get current user failed: %v", err)
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "internal server error")
		return
	}

	c.JSON(http.StatusOK, toUserResponse(u))
}
