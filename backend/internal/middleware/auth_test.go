package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/toshiki-1221/expense-workflow/internal/domain"
)

type fakeParser struct{}

func (fakeParser) Parse(token string) (int64, error) {
	if token == "good" {
		return 1, nil
	}
	return 0, errors.New("bad token")
}

type fakeLoader map[int64]*domain.User

func (f fakeLoader) CurrentUser(_ context.Context, id int64) (*domain.User, error) {
	if u, ok := f[id]; ok {
		return u, nil
	}
	return nil, domain.ErrUnauthorized
}

func newRouter(handlers ...gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/", append(handlers, func(c *gin.Context) {
		id, _ := UserID(c)
		c.JSON(http.StatusOK, gin.H{"user_id": id})
	})...)
	return r
}

func TestAuthenticate(t *testing.T) {
	r := newRouter(Authenticate(fakeParser{}))

	tests := []struct {
		name          string
		header        string
		wantStatus    int
		wantChallenge string
	}{
		{"正しいトークン", "Bearer good", http.StatusOK, ""},
		{"スキーム名は大文字小文字を区別しない", "bearer good", http.StatusOK, ""},
		{"ヘッダーなし", "", http.StatusUnauthorized, `Bearer realm="api"`},
		{"Bearer だけ", "Bearer ", http.StatusUnauthorized, `Bearer realm="api"`},
		{"別のスキーム", "Basic good", http.StatusUnauthorized, `Bearer realm="api"`},
		{"無効なトークン", "Bearer bad", http.StatusUnauthorized, `Bearer realm="api", error="invalid_token"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d; want %d (body=%s)", w.Code, tt.wantStatus, w.Body)
			}
			if got := w.Header().Get("WWW-Authenticate"); got != tt.wantChallenge {
				t.Fatalf("WWW-Authenticate = %q; want %q", got, tt.wantChallenge)
			}
		})
	}
}

func TestRequireRole(t *testing.T) {
	tests := []struct {
		name       string
		user       *domain.User
		wantStatus int
	}{
		{"許可されたロール", &domain.User{ID: 1, Role: domain.RoleAccounting}, http.StatusOK},
		{"許可されていないロール", &domain.User{ID: 1, Role: domain.RoleMember}, http.StatusForbidden},
		{"ユーザーが削除済み", nil, http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := fakeLoader{}
			if tt.user != nil {
				loader[tt.user.ID] = tt.user
			}
			r := newRouter(Authenticate(fakeParser{}), RequireRole(loader, domain.RoleAccounting))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Authorization", "Bearer good")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d; want %d (body=%s)", w.Code, tt.wantStatus, w.Body)
			}
		})
	}
}
