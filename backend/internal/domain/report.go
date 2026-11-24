package domain

import (
	"errors"
	"slices"
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

// CheckAction は viewer が action を実行できるかを確かめ、できないときは理由に合ったエラーを返す。
// 判定そのものは AllowedActions に任せ、ここではエラーの種類（HTTPステータス）を決めるだけにする。
//   - 見る権限がない        → ErrNotFound（存在を隠す）
//   - 見られるが担当ではない → ErrForbidden
//   - 担当だが状態が合わない → ErrInvalidStatus
func (r *Report) CheckAction(viewer *User, action ReportAction) error {
	if !r.CanBeViewedBy(viewer) {
		return ErrNotFound
	}
	if slices.Contains(r.AllowedActions(viewer), action) {
		return nil
	}

	isOwner := r.UserID == viewer.ID
	switch action {
	case ReportActionEdit, ReportActionDelete, ReportActionSubmit:
		if !isOwner {
			return ErrForbidden
		}
		if action == ReportActionSubmit && r.Status.Editable() && viewer.ApproverID == nil {
			return ErrNoApprover
		}
	case ReportActionApprove, ReportActionReject:
		if r.ApproverID == nil || *r.ApproverID != viewer.ID {
			return ErrForbidden
		}
	case ReportActionPay:
		if viewer.Role != RoleAccounting {
			return ErrForbidden
		}
	}
	return ErrInvalidStatus
}

// ReportSummary は一覧に出す申請の要約を表す。合計と件数は明細から集計した値。
type ReportSummary struct {
	ID          int64
	UserID      int64
	UserName    string // 承認待ち一覧のときだけ入る（自分の一覧では本人なので不要）
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
	ID          int64 // 更新のときだけ使う。0 は新しい明細（作成のときは常に 0）
	CategoryID  int64
	UsedOn      time.Time
	Amount      int64
	Description string
}

// ItemChanges は申請の明細をまとめて送られたときに、DBへ反映する差分を表す。
type ItemChanges struct {
	Create []ReportItemInput // id なし → 追加
	Update []ReportItemInput // id あり → 更新
	Delete []int64           // いまあるのに送られてこなかった → 削除
}

// DiffItems は、いまの明細（existing）と画面から送られた明細（inputs）を比べて差分を返す。
// 全部消して入れ直すと明細IDが変わり、あとで領収書を紐づけるときに困るので、差分で反映する。
// この申請にない id や、同じ id が2回出てきたときは ErrUnknownItem を返す。
func DiffItems(existing []ReportItem, inputs []ReportItemInput) (ItemChanges, error) {
	remaining := make(map[int64]bool, len(existing)) // まだ送られてきていない既存の明細
	for _, it := range existing {
		remaining[it.ID] = true
	}

	changes := ItemChanges{Create: []ReportItemInput{}, Update: []ReportItemInput{}, Delete: []int64{}}
	for _, in := range inputs {
		if in.ID == 0 {
			changes.Create = append(changes.Create, in)
			continue
		}
		if !remaining[in.ID] {
			return ItemChanges{}, ErrUnknownItem
		}
		delete(remaining, in.ID) // 2回目に同じ id が来たら上で弾かれる
		changes.Update = append(changes.Update, in)
	}

	// 元の並び順で削除対象を並べる（結果を安定させるため map の順に頼らない）
	for _, it := range existing {
		if remaining[it.ID] {
			changes.Delete = append(changes.Delete, it.ID)
		}
	}
	return changes, nil
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
	ErrForbidden       = errors.New("forbidden")                // 見られるが、その操作の担当ではない
	ErrInvalidStatus   = errors.New("invalid report status")    // いまの状態ではその操作ができない（同時操作も含む）
	ErrNoApprover      = errors.New("approver is not assigned") // 承認者が決まっていないので申請できない
	ErrUnknownItem     = errors.New("unknown report item")      // この申請にない明細 id が送られてきた
)

// ValidationError は入力不正の理由を持つエラー。Message はそのまま画面に出せる文言にする。
// errors.Is(err, ErrInvalidInput) で判定できる。
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

// Is は errors.Is で ErrInvalidInput と一致させる。
func (e *ValidationError) Is(target error) bool { return target == ErrInvalidInput }
