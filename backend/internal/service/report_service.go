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
	maxReportTitleLen     = 100 // expense_reports.title VARCHAR(100)
	maxItemDescriptionLen = 255 // expense_items.description VARCHAR(255)
)

// reportRepository は申請を保存・取得するRepositoryの振る舞いを定義する。
type reportRepository interface {
	CreateReport(ctx context.Context, userID int64, title string, items []domain.ReportItemInput) (*domain.Report, error)
	ListReports(ctx context.Context, userID int64, status *domain.ReportStatus) ([]domain.ReportSummary, error)
	GetReport(ctx context.Context, id int64) (*domain.Report, error)
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
	viewer, err := s.users.FindByID(ctx, userID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, domain.ErrUnauthorized // トークンは正しいがユーザーが消えている
	}
	if err != nil {
		return nil, err
	}

	report, err := s.repository.GetReport(ctx, reportID)
	if err != nil {
		return nil, err
	}
	if !report.CanBeViewedBy(viewer) {
		return nil, domain.ErrNotFound
	}

	return &ReportDetail{Report: report, AllowedActions: report.AllowedActions(viewer)}, nil
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
