// Package middleware はGinのミドルウェア（認証・認可など）を提供する。
package middleware

import (
	"context"
	"errors"
	"log"
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/toshiki-1221/expense-workflow/internal/domain"
	"github.com/toshiki-1221/expense-workflow/internal/handler/response"
)

// ctxKey はこのパッケージだけが作れる型。文字列キーだと他のミドルウェアと衝突しうる。
type ctxKey int

const (
	keyUserID ctxKey = iota
	keyUser
)

// TokenParser はトークンを検証してユーザーIDを返す振る舞いを定義する。
type TokenParser interface {
	Parse(token string) (int64, error)
}

// Authenticate は Bearer トークンを検証し、ユーザーIDを Context に入れる。
func Authenticate(tp TokenParser) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := bearerToken(c.GetHeader("Authorization"))
		if !ok {
			// RFC 6750: トークンが無いときはエラーコードを付けない
			c.Header("WWW-Authenticate", `Bearer realm="api"`)
			response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "ログインしてください")
			return
		}
		userID, err := tp.Parse(token)
		if err != nil {
			// 期限切れ・改ざん・別鍵などを区別して返さない（理由はサーバーログで追う）
			log.Printf("invalid token: %v", err)
			c.Header("WWW-Authenticate", `Bearer realm="api", error="invalid_token"`)
			response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "ログインの有効期限が切れたか、無効です。再度ログインしてください")
			return
		}
		c.Set(keyUserID, userID)
		c.Next()
	}
}

// bearerToken は "Bearer <token>" からトークンを取り出す。
// 認証スキーム名は大文字小文字を区別しない（RFC 9110）。
func bearerToken(header string) (string, bool) {
	scheme, token, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.TrimSpace(token)
	return token, token != ""
}

// UserID は Authenticate が入れたユーザーIDを取り出す。Authenticate の後ろでだけ使える。
func UserID(c *gin.Context) (int64, bool) {
	v, ok := c.Get(keyUserID)
	if !ok {
		return 0, false
	}
	id, ok := v.(int64)
	return id, ok
}

// CurrentUser は RequireRole が入れたユーザーを取り出す。RequireRole の後ろでだけ使える。
func CurrentUser(c *gin.Context) (*domain.User, bool) {
	v, ok := c.Get(keyUser)
	if !ok {
		return nil, false
	}
	u, ok := v.(*domain.User)
	return u, ok
}

// CurrentUserLoader はユーザーIDから最新のユーザー情報を取得する振る舞いを定義する。
type CurrentUserLoader interface {
	CurrentUser(ctx context.Context, userID int64) (*domain.User, error)
}

// RequireRole は指定ロールのユーザーだけ通す。Authenticate の後ろに付ける。
// ロールはトークンに入れず、毎回DBの最新値で判定する。
func RequireRole(users CurrentUserLoader, roles ...domain.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := UserID(c)
		if !ok {
			response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "ログインしてください")
			return
		}
		u, err := users.CurrentUser(c.Request.Context(), id)
		if errors.Is(err, domain.ErrUnauthorized) {
			response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "ログインしてください")
			return
		}
		if err != nil {
			log.Printf("load current user failed: %v", err)
			response.Error(c, http.StatusInternalServerError, response.CodeInternal, "internal server error")
			return
		}
		if !slices.Contains(roles, u.Role) {
			response.Error(c, http.StatusForbidden, response.CodeForbidden, "この操作を行う権限がありません")
			return
		}
		c.Set(keyUser, u)
		c.Next()
	}
}
