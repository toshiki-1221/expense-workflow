package handler

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/toshiki-1221/expense-workflow/internal/domain"
	"github.com/toshiki-1221/expense-workflow/internal/handler/response"
)

// categoryService は科目一覧を提供するServiceの振る舞いを定義する。
type categoryService interface {
	ListCategories(ctx context.Context) ([]domain.Category, error)
}

// CategoryHandler は科目一覧APIのHTTP処理を担当する。
type CategoryHandler struct {
	service categoryService
}

// NewCategoryHandler は科目一覧Handlerを作成する。
func NewCategoryHandler(service categoryService) *CategoryHandler {
	return &CategoryHandler{service: service}
}

// categoryResponse は科目一覧APIのレスポンス項目を表す。
type categoryResponse struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	SortOrder int32  `json:"sort_order"`
}

// listCategoriesResponse は科目一覧を {"categories": [...]} で包んだ形を表す。
type listCategoriesResponse struct {
	Categories []categoryResponse `json:"categories"`
}

// List は科目一覧をJSONで返す。
func (h *CategoryHandler) List(c *gin.Context) {
	categories, err := h.service.ListCategories(c.Request.Context())
	if err != nil {
		log.Printf("list categories failed: %v", err)
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "internal server error")
		return
	}

	items := make([]categoryResponse, 0, len(categories))
	for _, category := range categories {
		items = append(items, categoryResponse{
			ID:        category.ID,
			Name:      category.Name,
			SortOrder: category.SortOrder,
		})
	}

	c.JSON(http.StatusOK, listCategoriesResponse{Categories: items})
}
