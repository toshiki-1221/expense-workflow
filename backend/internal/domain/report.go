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

// Valid は定義済みのステータスかどうかを返す。
func (s ReportStatus) Valid() bool {
	switch s {
	case ReportStatusDraft, ReportStatusSubmitted, ReportStatusApproved, ReportStatusRejected, ReportStatusPaid:
		return true
	}
	return false
}

// Editable は申請者が編集・再申請できるステータスか（draft と rejected）を返す。
func (s ReportStatus) Editable() bool {
	return s == ReportStatusDraft || s == ReportStatusRejected
}

// ReportAction は申請に対してできる操作（画面のボタン）を表す。
// approve などの状態変更は approval_logs.action と1対1にする。
type ReportAction string

const (
	ReportActionEdit    ReportAction = "edit"
	ReportActionDelete  ReportAction = "delete"
	ReportActionSubmit  ReportAction = "submit"
	ReportActionApprove ReportAction = "approve"
	ReportActionReject  ReportAction = "reject"
	ReportActionPay     ReportAction = "pay"
)

// Report は経費申請（明細・履歴つき）を表す。
type Report struct {
	ID           int64
	UserID       int64
	UserName     string // 詳細取得のときだけ入る
	ApproverID   *int64 // 申請時に決まる。下書き中は nil
	ApproverName *string
	Title        string
	Status       ReportStatus
	SubmittedAt  *time.Time // 下書き中は nil
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Items        []ReportItem
	Logs         []ApprovalLog
}

// TotalAmount は明細の合計金額を返す（DBには持たない）。
func (r *Report) TotalAmount() int64 {
	var total int64
	for _, it := range r.Items {
		total += it.Amount
	}
	return total
}

// CanBeViewedBy は viewer がこの申請を見てよいかを返す。
// 申請者本人・この申請の承認者・経理だけが見られる。
func (r *Report) CanBeViewedBy(viewer *User) bool {
	switch {
	case r.UserID == viewer.ID:
		return true
	case r.ApproverID != nil && *r.ApproverID == viewer.ID:
		return true
	case viewer.Role == RoleAccounting:
		return true
	}
	return false
}

// AllowedActions は viewer がいまこの申請に対して押せるボタンを返す。
// 画面の出し分けと、操作APIでの権限チェックの両方でこの関数を使う（ルールを1か所にまとめる）。
func (r *Report) AllowedActions(viewer *User) []ReportAction {
	actions := []ReportAction{}
	isOwner := r.UserID == viewer.ID
	isApprover := r.ApproverID != nil && *r.ApproverID == viewer.ID

	if isOwner && r.Status.Editable() {
		actions = append(actions, ReportActionEdit)
		// 承認者が決まっていない人は申請できない（SubmitExpenseReport も同じ条件）
		if viewer.ApproverID != nil {
			actions = append(actions, ReportActionSubmit)
		}
	}
	if isOwner && r.Status == ReportStatusDraft {
		actions = append(actions, ReportActionDelete)
	}
	if isApprover && r.Status == ReportStatusSubmitted {
		actions = append(actions, ReportActionApprove, ReportActionReject)
	}
	if viewer.Role == RoleAccounting && r.Status == ReportStatusApproved {
		actions = append(actions, ReportActionPay)
	}
	return actions
}

// ReportSummary は一覧に出す申請の要約を表す。合計と件数は明細から集計した値。
type ReportSummary struct {
	ID          int64
	Title       string
	Status      ReportStatus
	TotalAmount int64
	ItemCount   int64
	SubmittedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ReportItem は申請の明細を表す。
type ReportItem struct {
	ID           int64
	ReportID     int64
	CategoryID   int64
	CategoryName string // 詳細取得のときだけ入る
	UsedOn       time.Time
	Amount       int64 // 円
	Description  string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ReportItemInput は明細を作成・更新するときの入力を表す。
type ReportItemInput struct {
	CategoryID  int64
	UsedOn      time.Time
	Amount      int64
	Description string
}

// ApprovalLog は申請の状態変更の履歴（誰が・いつ・何をしたか）を表す。
type ApprovalLog struct {
	ID         int64
	ActorID    int64
	ActorName  string
	Action     ReportAction // submit / approve / reject / pay
	FromStatus ReportStatus
	ToStatus   ReportStatus
	Comment    *string // 差し戻しのときは必須
	CreatedAt  time.Time
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
