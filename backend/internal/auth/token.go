package auth

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ErrInvalidToken は署名・期限・発行者・宛先のいずれかが不正なトークンを表す。
var ErrInvalidToken = errors.New("invalid token")

const (
	issuer   = "expense-workflow-api" // 誰が発行したか
	audience = "expense-workflow-web" // 誰向けのトークンか
)

// TokenManager はJWTの発行と検証を行う。
type TokenManager struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time // テストで時刻を差し替えるため
}

// NewTokenManager はHS256で署名するTokenManagerを作成する。
func NewTokenManager(secret string, ttl time.Duration) *TokenManager {
	return &TokenManager{secret: []byte(secret), ttl: ttl, now: time.Now}
}

// Issue はユーザーIDを sub に入れたトークンを発行する。
// 中身は誰でも読めるので、IDと期限以外（メール・ロールなど）は入れない。
func (m *TokenManager) Issue(userID int64) (string, error) {
	now := m.now()
	claims := jwt.RegisteredClaims{
		Subject:   strconv.FormatInt(userID, 10),
		Issuer:    issuer,
		Audience:  jwt.ClaimStrings{audience},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

// Parse は署名・アルゴリズム・発行者・宛先・有効期限を検証し、ユーザーIDを返す。
func (m *TokenManager) Parse(tokenString string) (int64, error) {
	claims := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(tokenString, claims,
		func(*jwt.Token) (any, error) { return m.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), // alg=none 等を拒否
		jwt.WithIssuer(issuer),
		jwt.WithAudience(audience), // 別サービス向けのトークンを拒否（RFC 8725）
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(), // iat が未来のトークンを拒否
		jwt.WithTimeFunc(m.now),
	)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	id, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil || id <= 0 {
		return 0, ErrInvalidToken
	}
	return id, nil
}
