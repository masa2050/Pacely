// Package handler は HTTP リクエストの受付とレスポンス整形を担当する(3層構成の最上層)。
package handler

import (
	"net/http"

	appmw "github.com/masa2050/pacely/backend/internal/middleware"
	"github.com/labstack/echo/v4"
)

type UserHandler struct{}

func NewUserHandler() *UserHandler {
	return &UserHandler{}
}

// GetMe は GET /users/me。
// フェーズ1タスク1の時点では、JWT 検証が通っているかの確認用に
// トークンから取り出した user_id をそのまま返す。
// タスク2で users テーブルと接続し、region などのプロフィールを返すように差し替える。
func (h *UserHandler) GetMe(c echo.Context) error {
	userID, _ := c.Get(appmw.ContextUserIDKey).(string)
	return c.JSON(http.StatusOK, echo.Map{
		"id": userID,
	})
}
