package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"

	appmw "github.com/masa2050/pacely/backend/internal/middleware"
	"github.com/masa2050/pacely/backend/internal/service"
)

type AdviceHandler struct {
	service *service.AdviceService
}

func NewAdviceHandler(service *service.AdviceService) *AdviceHandler {
	return &AdviceHandler{service: service}
}

// adviceToError はservice層のエラーをdocs/api.md 4.のステータスコードに変換する。
// AI API・天候APIの呼び出し失敗もここでは一律500として扱う(docs/api.md 4.に明記の方針)。
func adviceToError(err error) error {
	return echo.NewHTTPError(http.StatusInternalServerError, "AI提案の取得に失敗しました: "+err.Error())
}

// GetLatest は GET /advices/latest。
// docs/api.md 3.の生成判定ロジック(記録3件未満・目標未設定・24時間以内キャッシュ)は
// すべてservice層に任せ、handlerは結果をそのまま返すだけにする
// (docs/architecture.md 3.のレイヤー責務分離)。
func (h *AdviceHandler) GetLatest(c echo.Context) error {
	userID, _ := c.Get(appmw.ContextUserIDKey).(string)

	result, err := h.service.GetLatest(c.Request().Context(), userID)
	if err != nil {
		return adviceToError(err)
	}
	return c.JSON(http.StatusOK, result)
}

// List は GET /advices(過去の提案履歴一覧)。
func (h *AdviceHandler) List(c echo.Context) error {
	userID, _ := c.Get(appmw.ContextUserIDKey).(string)

	advices, err := h.service.List(c.Request().Context(), userID)
	if err != nil {
		return adviceToError(err)
	}
	return c.JSON(http.StatusOK, advices)
}
