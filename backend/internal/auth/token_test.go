package auth

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestTokenManager(t *testing.T) {
	secret := strings.Repeat("s", 32)
	base := time.Date(2026, 11, 21, 9, 0, 0, 0, time.UTC)

	newManager := func(secret string) *TokenManager {
		m := NewTokenManager(secret, time.Hour)
		m.now = func() time.Time { return base }
		return m
	}
	signed := func(t *testing.T, claims jwt.RegisteredClaims) string {
		t.Helper()
		tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
		if err != nil {
			t.Fatal(err)
		}
		return tok
	}
	validClaims := func() jwt.RegisteredClaims {
		return jwt.RegisteredClaims{
			Subject:   "42",
			Issuer:    issuer,
			Audience:  jwt.ClaimStrings{audience},
			IssuedAt:  jwt.NewNumericDate(base),
			ExpiresAt: jwt.NewNumericDate(base.Add(time.Hour)),
		}
	}

	t.Run("発行したトークンを検証できる", func(t *testing.T) {
		m := newManager(secret)
		tok, err := m.Issue(42)
		if err != nil {
			t.Fatal(err)
		}
		got, err := m.Parse(tok)
		if err != nil || got != 42 {
			t.Fatalf("Parse() = %d, %v; want 42, nil", got, err)
		}
	})

	rejectCases := []struct {
		name  string
		token func(t *testing.T) string
		at    time.Time
	}{
		{
			name: "期限切れ",
			token: func(t *testing.T) string {
				tok, _ := newManager(secret).Issue(42)
				return tok
			},
			at: base.Add(2 * time.Hour),
		},
		{
			name: "別の鍵で署名",
			token: func(t *testing.T) string {
				tok, _ := newManager(strings.Repeat("x", 32)).Issue(42)
				return tok
			},
		},
		{
			name: "宛先（aud）が違う",
			token: func(t *testing.T) string {
				c := validClaims()
				c.Audience = jwt.ClaimStrings{"other-service"}
				return signed(t, c)
			},
		},
		{
			name: "発行者（iss）が違う",
			token: func(t *testing.T) string {
				c := validClaims()
				c.Issuer = "other-issuer"
				return signed(t, c)
			},
		},
		{
			name: "exp が無い",
			token: func(t *testing.T) string {
				c := validClaims()
				c.ExpiresAt = nil
				return signed(t, c)
			},
		},
		{
			name: "iat が未来",
			token: func(t *testing.T) string {
				c := validClaims()
				c.IssuedAt = jwt.NewNumericDate(base.Add(30 * time.Minute))
				return signed(t, c)
			},
		},
		{
			name: "sub が数値でない",
			token: func(t *testing.T) string {
				c := validClaims()
				c.Subject = "abc"
				return signed(t, c)
			},
		},
		{
			name: "alg=none",
			token: func(t *testing.T) string {
				tok, err := jwt.NewWithClaims(jwt.SigningMethodNone, validClaims()).
					SignedString(jwt.UnsafeAllowNoneSignatureType)
				if err != nil {
					t.Fatal(err)
				}
				return tok
			},
		},
		{
			name:  "形式が壊れている",
			token: func(*testing.T) string { return "not-a-jwt" },
		},
	}
	for _, tt := range rejectCases {
		t.Run(tt.name+"は拒否", func(t *testing.T) {
			tok := tt.token(t)
			m := newManager(secret)
			if !tt.at.IsZero() {
				m.now = func() time.Time { return tt.at }
			}
			if _, err := m.Parse(tok); !errors.Is(err, ErrInvalidToken) {
				t.Fatalf("err = %v; want ErrInvalidToken", err)
			}
		})
	}
}
