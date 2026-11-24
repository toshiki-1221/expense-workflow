package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/toshiki-1221/expense-workflow/internal/db"
	"github.com/toshiki-1221/expense-workflow/internal/domain"
)

// pgForeignKeyViolation は外部キー制約違反のエラーコード。
const pgForeignKeyViolation = "23503"

// txBeginner はトランザクションを開始できるもの（*pgxpool.Pool など）。
type txBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// ReportRepository はsqlcを使って申請と明細を保存・取得する。
type ReportRepository struct {
	pool    txBeginner
	queries *db.Queries
}

// NewReportRepository は申請Repositoryを作成する。
// 複数テーブルをまとめて更新するので、トランザクション用に pool も受け取る。
func NewReportRepository(pool txBeginner, queries *db.Queries) *ReportRepository {
	return &ReportRepository{pool: pool, queries: queries}
}

// CreateReport は下書きの申請と明細を1トランザクションで作成する。
// 明細は0件でもよい。どれか1つでも失敗したら申請ごと作られない。
func (r *ReportRepository) CreateReport(ctx context.Context, userID int64, title string, items []domain.ReportItemInput) (*domain.Report, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	// 途中で return したら取り消す。Commit 済みなら何もしない。
	defer tx.Rollback(ctx) //nolint:errcheck

	q := r.queries.WithTx(tx)

	// 1. 申請を作る。id は DB が振り、RETURNING で返ってくる
	row, err := q.CreateExpenseReport(ctx, db.CreateExpenseReportParams{
		UserID: userID,
		Title:  title,
	})
	if err != nil {
		return nil, fmt.Errorf("create expense report: %w", err)
	}
	report := toDomainReport(row)

	// 2. 返ってきた申請IDを report_id にして明細を作る
	report.Items = make([]domain.ReportItem, 0, len(items))
	for _, in := range items {
		item, err := q.CreateExpenseItem(ctx, db.CreateExpenseItemParams{
			ReportID:    report.ID,
			CategoryID:  in.CategoryID,
			UsedOn:      in.UsedOn,
			Amount:      in.Amount,
			Description: in.Description,
			UserID:      userID,
		})
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation {
				return nil, domain.ErrInvalidCategory // 存在しない category_id
			}
			return nil, fmt.Errorf("create expense item: %w", err)
		}
		report.Items = append(report.Items, toDomainReportItem(item))
	}

	// 3. 全部成功したので確定する
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}
	return report, nil
}

// GetReport は申請を明細・履歴つきで取得する。誰が見てよいかはここでは判定しない（Service の担当）。
// 申請がないときは domain.ErrNotFound を返す。
func (r *ReportRepository) GetReport(ctx context.Context, id int64) (*domain.Report, error) {
	row, err := r.queries.GetExpenseReportDetail(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get expense report detail: %w", err)
	}

	itemRows, err := r.queries.ListExpenseReportItems(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list expense report items: %w", err)
	}
	logRows, err := r.queries.ListExpenseReportApprovalLogs(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list expense report approval logs: %w", err)
	}

	report := &domain.Report{
		ID:           row.ID,
		UserID:       row.UserID,
		UserName:     row.UserName,
		ApproverID:   row.ApproverID,
		ApproverName: row.ApproverName,
		Title:        row.Title,
		Status:       domain.ReportStatus(row.Status),
		SubmittedAt:  row.SubmittedAt,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
		Items:        make([]domain.ReportItem, 0, len(itemRows)),
		Logs:         make([]domain.ApprovalLog, 0, len(logRows)),
	}
	for _, it := range itemRows {
		report.Items = append(report.Items, domain.ReportItem{
			ID:           it.ID,
			ReportID:     it.ReportID,
			CategoryID:   it.CategoryID,
			CategoryName: it.CategoryName,
			UsedOn:       it.UsedOn,
			Amount:       it.Amount,
			Description:  it.Description,
			CreatedAt:    it.CreatedAt,
			UpdatedAt:    it.UpdatedAt,
		})
	}
	for _, l := range logRows {
		report.Logs = append(report.Logs, domain.ApprovalLog{
			ID:         l.ID,
			ActorID:    l.ActorID,
			ActorName:  l.ActorName,
			Action:     domain.ReportAction(l.Action),
			FromStatus: domain.ReportStatus(l.FromStatus),
			ToStatus:   domain.ReportStatus(l.ToStatus),
			Comment:    l.Comment,
			CreatedAt:  l.CreatedAt,
		})
	}
	return report, nil
}

// ListReports は自分の申請一覧を新しい順に取得する。status が nil なら全ステータス。
func (r *ReportRepository) ListReports(ctx context.Context, userID int64, status *domain.ReportStatus) ([]domain.ReportSummary, error) {
	var statusArg *string
	if status != nil {
		s := string(*status)
		statusArg = &s
	}

	rows, err := r.queries.ListMyExpenseReports(ctx, db.ListMyExpenseReportsParams{
		UserID: userID,
		Status: statusArg,
	})
	if err != nil {
		return nil, fmt.Errorf("list my expense reports: %w", err)
	}

	reports := make([]domain.ReportSummary, 0, len(rows))
	for _, row := range rows {
		reports = append(reports, domain.ReportSummary{
			ID:          row.ID,
			Title:       row.Title,
			Status:      domain.ReportStatus(row.Status),
			TotalAmount: row.TotalAmount,
			ItemCount:   row.ItemCount,
			SubmittedAt: row.SubmittedAt,
			CreatedAt:   row.CreatedAt,
			UpdatedAt:   row.UpdatedAt,
		})
	}
	return reports, nil
}

// toDomainReport はsqlcの申請行をドメインの申請に変換する（明細は含まない）。
func toDomainReport(row db.ExpenseReport) *domain.Report {
	return &domain.Report{
		ID:          row.ID,
		UserID:      row.UserID,
		ApproverID:  row.ApproverID,
		Title:       row.Title,
		Status:      domain.ReportStatus(row.Status),
		SubmittedAt: row.SubmittedAt,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}

// toDomainReportItem はsqlcの明細行をドメインの明細に変換する。
func toDomainReportItem(row db.ExpenseItem) domain.ReportItem {
	return domain.ReportItem{
		ID:          row.ID,
		ReportID:    row.ReportID,
		CategoryID:  row.CategoryID,
		UsedOn:      row.UsedOn,
		Amount:      row.Amount,
		Description: row.Description,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}
