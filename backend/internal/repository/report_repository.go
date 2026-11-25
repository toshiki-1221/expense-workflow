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
			return nil, itemWriteError("create expense item", err)
		}
		report.Items = append(report.Items, toDomainReportItem(item))
	}

	// 3. 全部成功したので確定する
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}
	return report, nil
}

// UpdateReport は申請の件名と明細を1トランザクションで更新する（draft / rejected のときだけ）。
// 明細は差分で反映する：id なし＝追加、id あり＝更新、いまあるのに送られてこなかった＝削除。
// 状態が変わっていた・この申請にない明細 id があったときは domain.ErrInvalidStatus を返す（画面を開き直してもらう）。
func (r *ReportRepository) UpdateReport(ctx context.Context, reportID, userID int64, title string, items []domain.ReportItemInput) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	q := r.queries.WithTx(tx)

	// 1. 件名を更新する。この UPDATE で申請の行がロックされるので、
	//    コミットまでのあいだに申請や別の更新が割り込んでこない
	if _, err := q.UpdateExpenseReport(ctx, db.UpdateExpenseReportParams{
		Title:  title,
		ID:     reportID,
		UserID: userID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrInvalidStatus // 本人でない・申請中などに変わっていた
		}
		return fmt.Errorf("update expense report: %w", err)
	}

	// 2. ロックしたあとの明細と比べて差分を出す（Service で見た明細より新しい可能性がある）
	rows, err := q.ListExpenseReportItems(ctx, reportID)
	if err != nil {
		return fmt.Errorf("list expense report items: %w", err)
	}
	existing := make([]domain.ReportItem, 0, len(rows))
	for _, row := range rows {
		existing = append(existing, domain.ReportItem{ID: row.ID})
	}
	changes, err := domain.DiffItems(existing, items)
	if err != nil {
		return domain.ErrInvalidStatus // 別の更新で消えた明細を送ってきた
	}

	// 3. 削除 → 更新 → 追加 の順に反映する
	for _, id := range changes.Delete {
		if _, err := q.DeleteExpenseItem(ctx, db.DeleteExpenseItemParams{ID: id, ReportID: reportID, UserID: userID}); err != nil {
			return fmt.Errorf("delete expense item: %w", err)
		}
	}
	for _, in := range changes.Update {
		_, err := q.UpdateExpenseItem(ctx, db.UpdateExpenseItemParams{
			CategoryID:  in.CategoryID,
			UsedOn:      in.UsedOn,
			Amount:      in.Amount,
			Description: in.Description,
			ID:          in.ID,
			ReportID:    reportID,
			UserID:      userID,
		})
		if err != nil {
			return itemWriteError("update expense item", err)
		}
	}
	for _, in := range changes.Create {
		_, err := q.CreateExpenseItem(ctx, db.CreateExpenseItemParams{
			ReportID:    reportID,
			CategoryID:  in.CategoryID,
			UsedOn:      in.UsedOn,
			Amount:      in.Amount,
			Description: in.Description,
			UserID:      userID,
		})
		if err != nil {
			return itemWriteError("create expense item", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// DeleteReport は下書きの申請を削除する。明細は外部キーの ON DELETE CASCADE で一緒に消える。
// SQL 1本の条件付きDELETE（本人・draft のときだけ）なので、それだけで1トランザクションになる。
// 削除が0行のとき（直前に申請された・消されたなど）は domain.ErrInvalidStatus を返す。
func (r *ReportRepository) DeleteReport(ctx context.Context, reportID, userID int64) error {
	n, err := r.queries.DeleteDraftExpenseReport(ctx, db.DeleteDraftExpenseReportParams{
		ID:     reportID,
		UserID: userID,
	})
	if err != nil {
		return fmt.Errorf("delete draft expense report: %w", err)
	}
	if n == 0 {
		return domain.ErrInvalidStatus
	}
	return nil
}

// itemWriteError は明細の保存エラーを変換する。存在しない category_id なら domain.ErrInvalidCategory にする。
func itemWriteError(op string, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation {
		return domain.ErrInvalidCategory
	}
	return fmt.Errorf("%s: %w", op, err)
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

// SubmitReport は申請を「申請中」にし、履歴を1行追加する。
// SQL 1本（条件付きUPDATE + INSERT）なので、それだけで1トランザクションになる。
// 申請者の承認者を申請にコピーする（あとで上長が変わってもぶれない）。
// 条件（本人・draft か rejected・承認者がいる）に合わず更新が0行のときは domain.ErrInvalidStatus を返す。
func (r *ReportRepository) SubmitReport(ctx context.Context, reportID, userID int64) error {
	_, err := r.queries.SubmitExpenseReport(ctx, db.SubmitExpenseReportParams{
		ReportID: reportID,
		UserID:   userID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrInvalidStatus // 直前に別の操作で状態が変わった
	}
	if err != nil {
		return fmt.Errorf("submit expense report: %w", err)
	}
	return nil
}

// ApproveReport は申請を「承認済み」にし、履歴を1行追加する。
// SQL 1本（条件付きUPDATE + INSERT）なので、それだけで1トランザクションになる。
// 条件（この申請の承認者・申請中）に合わず更新が0行のときは domain.ErrInvalidStatus を返す。
func (r *ReportRepository) ApproveReport(ctx context.Context, reportID, approverID int64) error {
	_, err := r.queries.ApproveExpenseReport(ctx, db.ApproveExpenseReportParams{
		ReportID: reportID,
		ActorID:  approverID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrInvalidStatus // 直前に別の操作で状態が変わった（二重承認など）
	}
	if err != nil {
		return fmt.Errorf("approve expense report: %w", err)
	}
	return nil
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
			UserID:      row.UserID,
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

// ListPendingApprovals は自分（approverID）宛ての承認待ち（申請中）を、申請日時の古い順に取得する。
func (r *ReportRepository) ListPendingApprovals(ctx context.Context, approverID int64) ([]domain.ReportSummary, error) {
	rows, err := r.queries.ListPendingApprovals(ctx, approverID)
	if err != nil {
		return nil, fmt.Errorf("list pending approvals: %w", err)
	}

	reports := make([]domain.ReportSummary, 0, len(rows))
	for _, row := range rows {
		reports = append(reports, domain.ReportSummary{
			ID:          row.ID,
			UserID:      row.UserID,
			UserName:    row.UserName,
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
