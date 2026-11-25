package handler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/toshiki-1221/expense-workflow/internal/domain"
	"github.com/toshiki-1221/expense-workflow/internal/handler/response"
	"github.com/toshiki-1221/expense-workflow/internal/middleware"
	"github.com/toshiki-1221/expense-workflow/internal/service"
)

// dateLayout はAPIで日付を受け渡す形式（例: 2026-11-03）。
const dateLayout = "2006-01-02"

// reportService は経費申請のユースケースを提供するServiceの振る舞いを定義する。
type reportService interface {
	CreateDraft(ctx context.Context, userID int64, title string, items []domain.ReportItemInput) (*service.ReportDetail, error)
	ListMine(ctx context.Context, userID int64, status string) ([]domain.ReportSummary, error)
	GetDetail(ctx context.Context, userID, reportID int64) (*service.ReportDetail, error)
	Submit(ctx context.Context, userID, reportID int64) (*service.ReportDetail, error)
	Approve(ctx context.Context, userID, reportID int64) (*service.ReportDetail, error)
	Reject(ctx context.Context, userID, reportID int64, comment string) (*service.ReportDetail, error)
	Update(ctx context.Context, userID, reportID int64, title string, items []domain.ReportItemInput) (*service.ReportDetail, error)
	Delete(ctx context.Context, userID, reportID int64) error
	ListPendingApprovals(ctx context.Context, userID int64) ([]domain.ReportSummary, error)
}

// ReportHandler は経費申請APIのHTTP処理を担当する。
type ReportHandler struct {
	service reportService
}

// NewReportHandler は申請Handlerを作成する。
func NewReportHandler(service reportService) *ReportHandler {
	return &ReportHandler{service: service}
}

// reportItemRequest は明細1件分のリクエスト項目を表す。
// 必須・桁数のチェックは Service でまとめて行う。
type reportItemRequest struct {
	ID          int64  `json:"id"` // 更新APIで既存の明細を指すときだけ。省略・null は新しい明細
	CategoryID  int64  `json:"category_id"`
	UsedOn      string `json:"used_on"`
	Amount      int64  `json:"amount"`
	Description string `json:"description"`
}

// createReportRequest は下書き作成APIのリクエストボディを表す。
// user_id は受け取らない（トークンから取る）。items は省略・空配列でもよい。
type createReportRequest struct {
	Title string              `json:"title"`
	Items []reportItemRequest `json:"items"`
}

// personResponse は申請者・承認者・操作者を {id, name} で表す。
type personResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// reportItemResponse は明細のレスポンス項目を表す。
type reportItemResponse struct {
	ID           int64  `json:"id"`
	CategoryID   int64  `json:"category_id"`
	CategoryName string `json:"category_name"`
	UsedOn       string `json:"used_on"`
	Amount       int64  `json:"amount"`
	Description  string `json:"description"`
}

// approvalLogResponse は履歴1件分のレスポンス項目を表す。
type approvalLogResponse struct {
	Actor      personResponse `json:"actor"`
	Action     string         `json:"action"`
	FromStatus string         `json:"from_status"`
	ToStatus   string         `json:"to_status"`
	Comment    *string        `json:"comment"`
	CreatedAt  time.Time      `json:"created_at"`
}

// reportResponse は申請詳細のレスポンスを表す。作成APIも同じ形で返す。
type reportResponse struct {
	ID             int64                 `json:"id"`
	User           personResponse        `json:"user"`
	Approver       *personResponse       `json:"approver"` // 下書き中は null
	Title          string                `json:"title"`
	Status         string                `json:"status"`
	SubmittedAt    *time.Time            `json:"submitted_at"`
	TotalAmount    int64                 `json:"total_amount"`
	Items          []reportItemResponse  `json:"items"`
	Logs           []approvalLogResponse `json:"logs"`
	AllowedActions []string              `json:"allowed_actions"` // ログイン中の人が押せるボタン
	CreatedAt      time.Time             `json:"created_at"`
	UpdatedAt      time.Time             `json:"updated_at"`
}

// toReportResponse は申請詳細をレスポンス形式に変換する。
func toReportResponse(d *service.ReportDetail) reportResponse {
	r := d.Report

	items := make([]reportItemResponse, 0, len(r.Items))
	for _, it := range r.Items {
		items = append(items, reportItemResponse{
			ID:           it.ID,
			CategoryID:   it.CategoryID,
			CategoryName: it.CategoryName,
			UsedOn:       it.UsedOn.Format(dateLayout),
			Amount:       it.Amount,
			Description:  it.Description,
		})
	}

	logs := make([]approvalLogResponse, 0, len(r.Logs))
	for _, l := range r.Logs {
		logs = append(logs, approvalLogResponse{
			Actor:      personResponse{ID: l.ActorID, Name: l.ActorName},
			Action:     string(l.Action),
			FromStatus: string(l.FromStatus),
			ToStatus:   string(l.ToStatus),
			Comment:    l.Comment,
			CreatedAt:  l.CreatedAt,
		})
	}

	var approver *personResponse
	if r.ApproverID != nil && r.ApproverName != nil {
		approver = &personResponse{ID: *r.ApproverID, Name: *r.ApproverName}
	}

	actions := make([]string, 0, len(d.AllowedActions))
	for _, a := range d.AllowedActions {
		actions = append(actions, string(a))
	}

	return reportResponse{
		ID:             r.ID,
		User:           personResponse{ID: r.UserID, Name: r.UserName},
		Approver:       approver,
		Title:          r.Title,
		Status:         string(r.Status),
		SubmittedAt:    r.SubmittedAt,
		TotalAmount:    r.TotalAmount(),
		Items:          items,
		Logs:           logs,
		AllowedActions: actions,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
}

// reportSummaryResponse は一覧に出す申請1件分のレスポンス項目を表す。
type reportSummaryResponse struct {
	ID          int64      `json:"id"`
	Title       string     `json:"title"`
	Status      string     `json:"status"`
	TotalAmount int64      `json:"total_amount"`
	ItemCount   int64      `json:"item_count"`
	SubmittedAt *time.Time `json:"submitted_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// listReportsResponse は申請一覧を {"reports": [...]} で包んだ形を表す（あとで pagination を足せる）。
type listReportsResponse struct {
	Reports []reportSummaryResponse `json:"reports"`
}

// approvalSummaryResponse は承認待ち一覧の1件分を表す。自分の一覧と違い、誰の申請かを付ける。
type approvalSummaryResponse struct {
	ID          int64          `json:"id"`
	User        personResponse `json:"user"`
	Title       string         `json:"title"`
	Status      string         `json:"status"`
	TotalAmount int64          `json:"total_amount"`
	ItemCount   int64          `json:"item_count"`
	SubmittedAt *time.Time     `json:"submitted_at"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// listApprovalsResponse は承認待ち一覧を {"reports": [...]} で包んだ形を表す（自分の一覧と揃える）。
type listApprovalsResponse struct {
	Reports []approvalSummaryResponse `json:"reports"`
}

// toItemInputs はリクエストの明細をServiceへの入力に変換する。利用日の形式だけここで見る。
func toItemInputs(reqs []reportItemRequest) ([]domain.ReportItemInput, error) {
	items := make([]domain.ReportItemInput, 0, len(reqs))
	for i, req := range reqs {
		var usedOn time.Time
		if req.UsedOn != "" { // 空のときはゼロ値のまま渡し、Service で「入力してください」にする
			t, err := time.Parse(dateLayout, req.UsedOn)
			if err != nil {
				return nil, fmt.Errorf("%d件目の利用日は 2026-11-03 の形式で入力してください", i+1)
			}
			usedOn = t
		}
		items = append(items, domain.ReportItemInput{
			ID:          req.ID,
			CategoryID:  req.CategoryID,
			UsedOn:      usedOn,
			Amount:      req.Amount,
			Description: req.Description,
		})
	}
	return items, nil
}

// Create は明細つきの下書きを作成する。
// POST /api/v1/reports
func (h *ReportHandler) Create(c *gin.Context) {
	userID, ok := middleware.UserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "ログインしてください")
		return
	}

	var req createReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "リクエストの形式が正しくありません")
		return
	}

	items, err := toItemInputs(req.Items)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}

	report, err := h.service.CreateDraft(c.Request.Context(), userID, req.Title, items)
	var verr *domain.ValidationError
	switch {
	case errors.As(err, &verr):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, verr.Message)
		return
	case errors.Is(err, domain.ErrInvalidCategory):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "存在しない科目が指定されています")
		return
	case errors.Is(err, domain.ErrUnauthorized):
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "ログインしてください")
		return
	case err != nil:
		log.Printf("create report failed: %v", err)
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "internal server error")
		return
	}

	c.JSON(http.StatusCreated, toReportResponse(report))
}

// List は自分の申請一覧を返す。?status= でステータスを絞り込める。
// GET /api/v1/reports
func (h *ReportHandler) List(c *gin.Context) {
	userID, ok := middleware.UserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "ログインしてください")
		return
	}

	reports, err := h.service.ListMine(c.Request.Context(), userID, c.Query("status"))
	var verr *domain.ValidationError
	switch {
	case errors.As(err, &verr):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, verr.Message)
		return
	case err != nil:
		log.Printf("list reports failed: %v", err)
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "internal server error")
		return
	}

	items := make([]reportSummaryResponse, 0, len(reports))
	for _, r := range reports {
		items = append(items, reportSummaryResponse{
			ID:          r.ID,
			Title:       r.Title,
			Status:      string(r.Status),
			TotalAmount: r.TotalAmount,
			ItemCount:   r.ItemCount,
			SubmittedAt: r.SubmittedAt,
			CreatedAt:   r.CreatedAt,
			UpdatedAt:   r.UpdatedAt,
		})
	}
	c.JSON(http.StatusOK, listReportsResponse{Reports: items})
}

// ListApprovals は自分宛ての承認待ち（申請中）を、申請日時の古い順に返す。
// 承認者になっていない人は空の一覧になる。
// GET /api/v1/approvals
func (h *ReportHandler) ListApprovals(c *gin.Context) {
	userID, ok := middleware.UserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "ログインしてください")
		return
	}

	reports, err := h.service.ListPendingApprovals(c.Request.Context(), userID)
	if err != nil {
		log.Printf("list approvals failed: %v", err)
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "internal server error")
		return
	}

	items := make([]approvalSummaryResponse, 0, len(reports))
	for _, r := range reports {
		items = append(items, approvalSummaryResponse{
			ID:          r.ID,
			User:        personResponse{ID: r.UserID, Name: r.UserName},
			Title:       r.Title,
			Status:      string(r.Status),
			TotalAmount: r.TotalAmount,
			ItemCount:   r.ItemCount,
			SubmittedAt: r.SubmittedAt,
			CreatedAt:   r.CreatedAt,
			UpdatedAt:   r.UpdatedAt,
		})
	}
	c.JSON(http.StatusOK, listApprovalsResponse{Reports: items})
}

// Get は申請の詳細（明細・履歴・押せるボタン）を返す。
// 他人の申請や存在しないIDは、区別せず 404 にする。
// GET /api/v1/reports/:id
func (h *ReportHandler) Get(c *gin.Context) {
	userID, reportID, ok := userAndReportID(c)
	if !ok {
		return
	}

	detail, err := h.service.GetDetail(c.Request.Context(), userID, reportID)
	if err != nil {
		writeReportError(c, err, "get report")
		return
	}
	c.JSON(http.StatusOK, toReportResponse(detail))
}

// Update は申請の件名と明細を更新する（draft / rejected のみ）。
// ボディは作成と同じ形で、既存の明細には id を付ける。送られてこなかった明細は削除される。
// PUT /api/v1/reports/:id
func (h *ReportHandler) Update(c *gin.Context) {
	userID, reportID, ok := userAndReportID(c)
	if !ok {
		return
	}

	var req createReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "リクエストの形式が正しくありません")
		return
	}
	items, err := toItemInputs(req.Items)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}

	detail, err := h.service.Update(c.Request.Context(), userID, reportID, req.Title, items)
	if err != nil {
		writeReportError(c, err, "update report")
		return
	}
	c.JSON(http.StatusOK, toReportResponse(detail))
}

// Delete は下書きの申請を明細ごと削除する（draft のみ）。成功したら 204 で本文なし。
// DELETE /api/v1/reports/:id
func (h *ReportHandler) Delete(c *gin.Context) {
	userID, reportID, ok := userAndReportID(c)
	if !ok {
		return
	}

	if err := h.service.Delete(c.Request.Context(), userID, reportID); err != nil {
		writeReportError(c, err, "delete report")
		return
	}
	c.Status(http.StatusNoContent)
}

// Submit は申請を提出する（draft / rejected → submitted）。ボディは不要。
// POST /api/v1/reports/:id/submit
func (h *ReportHandler) Submit(c *gin.Context) {
	userID, reportID, ok := userAndReportID(c)
	if !ok {
		return
	}

	detail, err := h.service.Submit(c.Request.Context(), userID, reportID)
	if err != nil {
		writeReportError(c, err, "submit report")
		return
	}
	c.JSON(http.StatusOK, toReportResponse(detail))
}

// Approve は申請を承認する（submitted → approved）。ボディは不要。
// POST /api/v1/reports/:id/approve
func (h *ReportHandler) Approve(c *gin.Context) {
	userID, reportID, ok := userAndReportID(c)
	if !ok {
		return
	}

	detail, err := h.service.Approve(c.Request.Context(), userID, reportID)
	if err != nil {
		writeReportError(c, err, "approve report")
		return
	}
	c.JSON(http.StatusOK, toReportResponse(detail))
}

// rejectReportRequest は差し戻しAPIのリクエストボディを表す。必須チェックは Service で行う。
type rejectReportRequest struct {
	Comment string `json:"comment"`
}

// Reject は申請を差し戻す（submitted → rejected）。理由（comment）は必須。
// POST /api/v1/reports/:id/reject
func (h *ReportHandler) Reject(c *gin.Context) {
	userID, reportID, ok := userAndReportID(c)
	if !ok {
		return
	}

	var req rejectReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "リクエストの形式が正しくありません")
		return
	}

	detail, err := h.service.Reject(c.Request.Context(), userID, reportID, req.Comment)
	if err != nil {
		writeReportError(c, err, "reject report")
		return
	}
	c.JSON(http.StatusOK, toReportResponse(detail))
}

// userAndReportID はログイン中のユーザーIDと、パスの :id を取り出す。
// 取り出せないときはエラーレスポンスを書いて ok=false を返す。
func userAndReportID(c *gin.Context) (userID, reportID int64, ok bool) {
	userID, ok = middleware.UserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "ログインしてください")
		return 0, 0, false
	}
	reportID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || reportID <= 0 {
		// 数字でない id も「その申請はない」として扱う
		response.Error(c, http.StatusNotFound, response.CodeNotFound, "申請が見つかりません")
		return 0, 0, false
	}
	return userID, reportID, true
}

// writeReportError は申請APIの業務エラーをHTTPステータスに変換して返す。
func writeReportError(c *gin.Context, err error, op string) {
	var verr *domain.ValidationError
	switch {
	case errors.As(err, &verr):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, verr.Message)
	case errors.Is(err, domain.ErrInvalidCategory):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "存在しない科目が指定されています")
	case errors.Is(err, domain.ErrNotFound):
		response.Error(c, http.StatusNotFound, response.CodeNotFound, "申請が見つかりません")
	case errors.Is(err, domain.ErrUnauthorized):
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "ログインしてください")
	case errors.Is(err, domain.ErrForbidden):
		response.Error(c, http.StatusForbidden, response.CodeForbidden, "この操作をする権限がありません")
	case errors.Is(err, domain.ErrNoApprover):
		response.Error(c, http.StatusConflict, response.CodeConflict, "承認者が設定されていないため申請できません。管理者に連絡してください")
	case errors.Is(err, domain.ErrInvalidStatus):
		response.Error(c, http.StatusConflict, response.CodeConflict, "この申請はいまの状態では操作できません。画面を再読み込みしてください")
	default:
		log.Printf("%s failed: %v", op, err)
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "internal server error")
	}
}
