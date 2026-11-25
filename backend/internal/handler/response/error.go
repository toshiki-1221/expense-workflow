// Package response はAPI共通のHTTPレスポンス形式を提供する。
// handler と middleware の両方から使う。
package response

import "github.com/gin-gonic/gin"

// エラーコードはAPI仕様のステータスごとの分類に合わせる。
const (
	CodeBadRequest   = "bad_request"
	CodeUnauthorized = "unauthorized"
	CodeForbidden    = "forbidden"
	CodeNotFound     = "not_found"
	CodeConflict     = "conflict"
	CodeInternal     = "internal_error"
)

// errorBody はエラーレスポンスの中身を表す。
type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// errorResponse は {"error": {"code": ..., "message": ...}} の形を表す。
type errorResponse struct {
	Error errorBody `json:"error"`
}

// Error はAPI仕様の形式でエラーを返し、以降のハンドラ実行を止める。
func Error(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, errorResponse{
		Error: errorBody{Code: code, Message: message},
	})
}
