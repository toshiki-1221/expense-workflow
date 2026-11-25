package domain

import "errors"

// Role はユーザーのロールを表す。users.role の CHECK 制約と合わせる。
type Role string

const (
	RoleMember     Role = "member"
	RoleAccounting Role = "accounting"
)

// User はログインユーザーを表す。
type User struct {
	ID           int64
	Email        string
	Name         string
	PasswordHash string // handler のレスポンスには絶対に載せない
	Role         Role
	ApproverID   *int64 // 自分の承認者。NULL＝いない
	IsApprover   bool   // 誰かの承認者になっているか（ナビの出し分けに使う）
}

// 業務エラー。handler でHTTPステータスに変換する。
var (
	ErrNotFound           = errors.New("not found")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUnauthorized       = errors.New("unauthorized")
)
