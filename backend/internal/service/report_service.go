package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/toshiki-1221/expense-workflow/internal/domain"
)

const (
	maxReportTitleLen     = 100  // expense_reports.title VARCHAR(100)
	maxItemDescriptionLen = 255  // expense_items.description VARCHAR(255)
	maxRejectCommentLen   = 1000 // approval_logs.comment は TEXT だが、画面で読める長さに抑える
)

// reportRepository は申請を保存・取得するRepositoryの振る舞いを定義する。
type reportRepository interface {
	CreateReport(ctx context.Context, userID int64, title string, items []domain.ReportItemInput) (*domain.Report, error)
	ListReports(ctx context.Context, userID int64, status *domain.ReportStatus) ([]domain.ReportSummary, error)
	GetReport(ctx context.Context, id int64) (*domain.Report, error)
	SubmitReport(ctx context.Context, reportID, userID int64) error
	ApproveReport(ctx context.Context, reportID, approverID int64) error
	RejectReport(ctx context.Context, reportID, approverID int64, comment string) error
	UpdateReport(ctx context.Context, reportID, userID int64, title string, items []domain.ReportItemInput) error
	DeleteReport(ctx context.Context, reportID, userID int64) error
	ListPendingApprovals(ctx context.Context, approverID int64) ([]domain.ReportSummary, error)
}

// userFinder はログイン中のユーザー（ロール・承認者）を取得する振る舞いを定義する。
type userFinder interface {
	FindByID(ctx context.Context, id int64) (*domain.User, error)
}

// ReportService は経費申請のユースケースを提供する。
type ReportService struct {
	repository reportRepository
	users      userFinder
}

// NewReportService は申請Serviceを作成する。
// 見てよいか・押せるボタンはロールで変わるので、ユーザーの取得元も受け取る。
func NewReportService(repository reportRepository, users userFinder) *ReportService {
	return &ReportService{repository: repository, users: users}
}

// ReportDetail は申請の詳細と、閲覧者が押せるボタンの一覧を表す。
type ReportDetail struct {
	Report         *domain.Report
	AllowedActions []domain.ReportAction
}

// CreateDraft は明細つきの下書きを作成し、詳細と同じ形で返す。明細は0件でもよい。
// 入力が不正なときは *domain.ValidationError を返す。
func (s *ReportService) CreateDraft(ctx context.Context, userID int64, title string, items []domain.ReportItemInput) (*ReportDetail, error) {
	title = strings.TrimSpace(title)
	if err := validateDraft(title, items); err != nil {
		return nil, err
	}
	for i, item := range items {
		if item.ID != 0 {
			return nil, &domain.ValidationError{Message: fmt.Sprintf("%d件目：新しく作るときは明細の id を指定できません", i+1)}
		}
	}
	created, err := s.repository.CreateReport(ctx, userID, title, items)
	if err != nil {
		return nil, err
	}
	// 科目名・申請者名・allowed_actions まで揃えるため、詳細を取り直す
	return s.GetDetail(ctx, userID, created.ID)
}

// GetDetail は申請の詳細を取得する。
// 見る権限がない申請は、存在を隠すため domain.ErrNotFound を返す（403 ではなく 404）。
func (s *ReportService) GetDetail(ctx context.Context, userID, reportID int64) (*ReportDetail, error) {
	viewer, report, err := s.load(ctx, userID, reportID)
	if err != nil {
		return nil, err
	}
	if !report.CanBeViewedBy(viewer) {
		return nil, domain.ErrNotFound
	}
	return &ReportDetail{Report: report, AllowedActions: report.AllowedActions(viewer)}, nil
}

// Submit は申請を提出し（draft / rejected → submitted）、提出後の詳細を返す。
// 押せるかどうかは詳細の allowed_actions と同じ関数（CheckAction）で判定する。
func (s *ReportService) Submit(ctx context.Context, userID, reportID int64) (*ReportDetail, error) {
	viewer, report, err := s.load(ctx, userID, reportID)
	if err != nil {
		return nil, err
	}
	if err := report.CheckAction(viewer, domain.ReportActionSubmit); err != nil {
		return nil, err
	}
	// 下書きは明細0件でもよいが、申請するときは1件以上必須
	if len(report.Items) == 0 {
		return nil, &domain.ValidationError{Message: "明細を1件以上追加してから申請してください"}
	}

	// 上のチェックのあとに別の操作で状態が変わっていても、SQL の条件付きUPDATEで弾かれる
	if err := s.repository.SubmitReport(ctx, reportID, userID); err != nil {
		return nil, err
	}
	return s.GetDetail(ctx, userID, reportID)
}

// Approve は申請を承認し（submitted → approved）、承認後の詳細を返す。
// 押せるのは、この申請の承認者だけ（申請した時点で決まった approver_id）。
func (s *ReportService) Approve(ctx context.Context, userID, reportID int64) (*ReportDetail, error) {
	viewer, report, err := s.load(ctx, userID, reportID)
	if err != nil {
		return nil, err
	}
	if err := report.CheckAction(viewer, domain.ReportActionApprove); err != nil {
		return nil, err
	}

	// 上のチェックのあとに別の操作で状態が変わっていても、SQL の条件付きUPDATEで弾かれる
	if err := s.repository.ApproveReport(ctx, reportID, userID); err != nil {
		return nil, err
	}
	return s.GetDetail(ctx, userID, reportID)
}

// Reject は申請を差し戻し（submitted → rejected）、差し戻し後の詳細を返す。
// 何を直せばよいか申請者に伝えるため、理由（comment）は必須。
// 差し戻された申請は、申請者が直して（Update）もう一度出す（Submit）。
func (s *ReportService) Reject(ctx context.Context, userID, reportID int64, comment string) (*ReportDetail, error) {
	viewer, report, err := s.load(ctx, userID, reportID)
	if err != nil {
		return nil, err
	}
	// 押せない人には、理由の入力ミスより先に 404 / 403 / 409 を返す
	if err := report.CheckAction(viewer, domain.ReportActionReject); err != nil {
		return nil, err
	}

	comment = strings.TrimSpace(comment)
	if comment == "" {
		return nil, &domain.ValidationError{Message: "差し戻しの理由を入力してください"}
	}
	if utf8.RuneCountInString(comment) > maxRejectCommentLen {
		return nil, &domain.ValidationError{Message: fmt.Sprintf("差し戻しの理由は%d文字以内で入力してください", maxRejectCommentLen)}
	}

	// 上のチェックのあとに別の操作で状態が変わっていても、SQL の条件付きUPDATEで弾かれる
	if err := s.repository.RejectReport(ctx, reportID, userID, comment); err != nil {
		return nil, err
	}
	return s.GetDetail(ctx, userID, reportID)
}

// Update は申請の件名と明細を更新し（draft / rejected のみ）、更新後の詳細を返す。
// 明細は申請ごとまとめて受け取り、差分で反映する（id なし＝追加、id あり＝更新、送られてこなかった＝削除）。
// 差し戻しの申請を直しても状態は rejected のまま。もう一度出すときは Submit を呼ぶ。
func (s *ReportService) Update(ctx context.Context, userID, reportID int64, title string, items []domain.ReportItemInput) (*ReportDetail, error) {
	viewer, report, err := s.load(ctx, userID, reportID)
	if err != nil {
		return nil, err
	}
	if err := report.CheckAction(viewer, domain.ReportActionEdit); err != nil {
		return nil, err
	}

	title = strings.TrimSpace(title)
	if err := validateDraft(title, items); err != nil {
		return nil, err
	}
	// この申請にない明細 id は、画面のバグか他人の明細を狙ったものなので 400 にする
	if _, err := domain.DiffItems(report.Items, items); err != nil {
		return nil, &domain.ValidationError{Message: "この申請にない明細が含まれています"}
	}

	// 上のチェックのあとに状態や明細が変わっていても、Repository がロックしてから比べ直す
	if err := s.repository.UpdateReport(ctx, reportID, userID, title, items); err != nil {
		return nil, err
	}
	return s.GetDetail(ctx, userID, reportID)
}

// Delete は下書きの申請を明細ごと削除する（draft のみ）。
// 一度でも申請した（差し戻しを含む）ものは履歴が残っているので消せない。
func (s *ReportService) Delete(ctx context.Context, userID, reportID int64) error {
	viewer, report, err := s.load(ctx, userID, reportID)
	if err != nil {
		return err
	}
	if err := report.CheckAction(viewer, domain.ReportActionDelete); err != nil {
		return err
	}
	// 上のチェックのあとに申請されていても、SQL の条件付きDELETEで弾かれる
	return s.repository.DeleteReport(ctx, reportID, userID)
}

// load はログイン中のユーザーと申請（明細・履歴つき）を取得する。
func (s *ReportService) load(ctx context.Context, userID, reportID int64) (*domain.User, *domain.Report, error) {
	viewer, err := s.users.FindByID(ctx, userID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, nil, domain.ErrUnauthorized // トークンは正しいがユーザーが消えている
	}
	if err != nil {
		return nil, nil, err
	}

	report, err := s.repository.GetReport(ctx, reportID)
	if err != nil {
		return nil, nil, err
	}
	return viewer, report, nil
}

// ListMine は自分の申請一覧を取得する。status が空なら全ステータス。
// 未定義のステータスが来たときは *domain.ValidationError を返す。
func (s *ReportService) ListMine(ctx context.Context, userID int64, status string) ([]domain.ReportSummary, error) {
	if status == "" {
		return s.repository.ListReports(ctx, userID, nil)
	}
	st := domain.ReportStatus(status)
	if !st.Valid() {
		return nil, &domain.ValidationError{Message: "status は draft / submitted / approved / rejected / paid のどれかを指定してください"}
	}
	return s.repository.ListReports(ctx, userID, &st)
}

// ListPendingApprovals は自分宛ての承認待ち一覧を取得する。
// 誰でも呼べる。承認者になっていない人は空の一覧になる（ロールでは弾かない）。
func (s *ReportService) ListPendingApprovals(ctx context.Context, userID int64) ([]domain.ReportSummary, error) {
	return s.repository.ListPendingApprovals(ctx, userID)
}

// validateDraft は件名と明細の必須・桁数をチェックする。
func validateDraft(title string, items []domain.ReportItemInput) error {
	invalid := func(format string, args ...any) error {
		return &domain.ValidationError{Message: fmt.Sprintf(format, args...)}
	}

	if title == "" {
		return invalid("件名を入力してください")
	}
	if utf8.RuneCountInString(title) > maxReportTitleLen {
		return invalid("件名は%d文字以内で入力してください", maxReportTitleLen)
	}

	for i, item := range items {
		n := i + 1
		if item.CategoryID <= 0 {
			return invalid("%d件目の科目を選択してください", n)
		}
		if item.UsedOn.IsZero() {
			return invalid("%d件目の利用日を入力してください", n)
		}
		if item.Amount < 1 {
			return invalid("%d件目の金額は1円以上で入力してください", n)
		}
		if utf8.RuneCountInString(item.Description) > maxItemDescriptionLen {
			return invalid("%d件目の内容は%d文字以内で入力してください", n, maxItemDescriptionLen)
		}
	}
	return nil
}
