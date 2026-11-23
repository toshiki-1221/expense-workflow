package service

import (
	"context"
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
}

// ReportService は経費申請のユースケースを提供する。
type ReportService struct {
	repository reportRepository
}

// NewReportService は申請Serviceを作成する。
func NewReportService(repository reportRepository) *ReportService {
	return &ReportService{repository: repository}
}

// CreateDraft は明細つきの下書きを作成する。明細は0件でもよい。
// 入力が不正なときは *domain.ValidationError を返す。
func (s *ReportService) CreateDraft(ctx context.Context, userID int64, title string, items []domain.ReportItemInput) (*domain.Report, error) {
	title = strings.TrimSpace(title)
	if err := validateDraft(title, items); err != nil {
		return nil, err
	}
	return s.repository.CreateReport(ctx, userID, title, items)
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
