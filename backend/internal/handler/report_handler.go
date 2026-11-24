package handler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/toshiki-1221/expense-workflow/internal/domain"
	"github.com/toshiki-1221/expense-workflow/internal/handler/response"
	"github.com/toshiki-1221/expense-workflow/internal/middleware"
)

// dateLayout はAPIで日付を受け渡す形式（例: 2026-11-03）。
const dateLayout = "2006-01-02"

// reportService は経費申請のユースケースを提供するServiceの振る舞いを定義する。
type reportService interface {
	CreateDraft(ctx context.Context, userID int64, title string, items []domain.ReportItemInput) (*domain.Report, error)
	ListMine(ctx context.Context, userID int64, status string) ([]domain.ReportSummary, error)
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

// reportItemResponse は明細のレスポンス項目を表す。
type reportItemResponse struct {
	ID          int64  `json:"id"`
	CategoryID  int64  `json:"category_id"`
	UsedOn      string `json:"used_on"`
	Amount      int64  `json:"amount"`
	Description string `json:"description"`
}

// reportResponse は申請（明細つき）のレスポンスを表す。
type reportResponse struct {
	ID          int64                `json:"id"`
	UserID      int64                `json:"user_id"`
	ApproverID  *int64               `json:"approver_id"`
	Title       string               `json:"title"`
	Status      string               `json:"status"`
	SubmittedAt *time.Time           `json:"submitted_at"`
	TotalAmount int64                `json:"total_amount"`
	Items       []reportItemResponse `json:"items"`
	CreatedAt   time.Time            `json:"created_at"`
	UpdatedAt   time.Time            `json:"updated_at"`
}

// toReportResponse はドメインの申請をレスポンス形式に変換する。合計は明細から計算する。
func toReportResponse(r *domain.Report) reportResponse {
	items := make([]reportItemResponse, 0, len(r.Items))
	var total int64
	for _, it := range r.Items {
		total += it.Amount
		items = append(items, reportItemResponse{
			ID:          it.ID,
			CategoryID:  it.CategoryID,
			UsedOn:      it.UsedOn.Format(dateLayout),
			Amount:      it.Amount,
			Description: it.Description,
		})
	}
	return reportResponse{
		ID:          r.ID,
		UserID:      r.UserID,
		ApproverID:  r.ApproverID,
		Title:       r.Title,
		Status:      string(r.Status),
		SubmittedAt: r.SubmittedAt,
		TotalAmount: total,
		Items:       items,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
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
