// Package handler は HTTP リクエストの受付とレスポンス整形を担当する(3層構成の最上層)。
package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"

	appmw "github.com/masa2050/pacely/backend/internal/middleware"
	"github.com/masa2050/pacely/backend/internal/service"
)

type UserHandler struct {
	service *service.UserService
}

func NewUserHandler(service *service.UserService) *UserHandler {
	return &UserHandler{service: service}
}

// GetMe は GET /users/me。
func (h *UserHandler) GetMe(c echo.Context) error {
	userID, _ := c.Get(appmw.ContextUserIDKey).(string)

	user, err := h.service.GetOrCreateMe(c.Request().Context(), userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "プロフィールの取得に失敗しました")
	}
	return c.JSON(http.StatusOK, user)
}

type updateMeRequest struct {
	Region string `json:"region"`
}

// UpdateMe は PUT /users/me。region(天候取得用の地域)を更新する。
func (h *UserHandler) UpdateMe(c echo.Context) error {
	userID, _ := c.Get(appmw.ContextUserIDKey).(string)

	var body updateMeRequest
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "リクエストボディが不正です")
	}
	if body.Region == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "region は必須です")
	}

	user, err := h.service.UpdateRegion(c.Request().Context(), userID, body.Region)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "プロフィールの更新に失敗しました")
	}
	return c.JSON(http.StatusOK, user)
}

// DeleteMe は DELETE /users/me(退会)。
// Pacely独自データの削除とSupabase Authアカウントの削除まで行う(docs/adr/014)。
func (h *UserHandler) DeleteMe(c echo.Context) error {
	userID, _ := c.Get(appmw.ContextUserIDKey).(string)

	if err := h.service.DeleteAccount(c.Request().Context(), userID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "退会処理に失敗しました")
	}
	return c.NoContent(http.StatusNoContent)
}
