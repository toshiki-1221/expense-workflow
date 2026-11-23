package domain

import (
	"errors"
	"time"
)

// ReportStatus は申請のステータスを表す。expense_reports.status の CHECK 制約と合わせる。
type ReportStatus string

const (
	ReportStatusDraft     ReportStatus = "draft"
	ReportStatusSubmitted ReportStatus = "submitted"
	ReportStatusApproved  ReportStatus = "approved"
	ReportStatusRejected  ReportStatus = "rejected"
	ReportStatusPaid      ReportStatus = "paid"
)

// Report は経費申請（明細つき）を表す。
type Report struct {
	ID          int64
	UserID      int64
	ApproverID  *int64 // 申請時に決まる。下書き中は nil
	Title       string
	Status      ReportStatus
	SubmittedAt *time.Time // 下書き中は nil
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Items       []ReportItem
}

// ReportItem は申請の明細を表す。
type ReportItem struct {
	ID          int64
	ReportID    int64
	CategoryID  int64
	UsedOn      time.Time
	Amount      int64 // 円
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ReportItemInput は明細を作成・更新するときの入力を表す。
type ReportItemInput struct {
	CategoryID  int64
	UsedOn      time.Time
	Amount      int64
	Description string
}

// 申請まわりの業務エラー。handler でHTTPステータスに変換する。
var (
	ErrInvalidCategory = errors.New("invalid category")
	ErrInvalidInput    = errors.New("invalid input")
)

// ValidationError は入力不正の理由を持つエラー。Message はそのまま画面に出せる文言にする。
// errors.Is(err, ErrInvalidInput) で判定できる。
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

// Is は errors.Is で ErrInvalidInput と一致させる。
func (e *ValidationError) Is(target error) bool { return target == ErrInvalidInput }
