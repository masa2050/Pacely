package handler

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	appmw "github.com/masa2050/pacely/backend/internal/middleware"
	"github.com/masa2050/pacely/backend/internal/repository"
	"github.com/masa2050/pacely/backend/internal/service"
)

type AdviceHandler struct {
	service *service.AdviceService
}

func NewAdviceHandler(service *service.AdviceService) *AdviceHandler {
	return &AdviceHandler{service: service}
}

// adviceToError はservice層のエラーをdocs/api.md 4.のステータスコードに変換する。
// AI API・天候APIの呼び出し失敗はservice.ErrExternalAPIとして分類され、
// 500で定型メッセージのみを返す(docs/implementation-plan.md 8-1)。
// *url.Errorやレスポンス本文にAPIキー・生JSONが含まれうるため、詳細はai_client.go/
// weather_client.go側でログにのみ出力済みで、ここではerr.Error()を一切使わない。
func adviceToError(err error) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "AI提案が見つかりません")
	case errors.Is(err, service.ErrForbidden):
		return echo.NewHTTPError(http.StatusForbidden, "他ユーザーのAI提案は操作できません")
	case errors.Is(err, service.ErrInvalidInput):
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrExternalAPI):
		// 詳細(APIキー込みのURL・レスポンス本文)はai_client.go/weather_client.go側で
		// 発生元に近い場所ですでにログ済みなので、ここでは定型メッセージのみ返す。
		return echo.NewHTTPError(http.StatusInternalServerError, "AI提案の生成に失敗しました。しばらくしてから再度お試しください")
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, "AI提案の取得に失敗しました: "+err.Error())
	}
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

// feedbackRequest は PUT /advices/{id}/feedback のリクエストボディ。
// is_helpful: true=役に立った / false=役に立たなかった。未送信はservice層で400にする。
type feedbackRequest struct {
	IsHelpful *bool   `json:"is_helpful"`
	Comment   *string `json:"comment"`
}

// SubmitFeedback は PUT /advices/{id}/feedback(フェーズ7-5、docs/adr/019)。
// 評価の上書きを許す冪等な操作なのでPUTにしている。
func (h *AdviceHandler) SubmitFeedback(c echo.Context) error {
	userID, _ := c.Get(appmw.ContextUserIDKey).(string)

	var req feedbackRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "リクエストボディが不正です")
	}

	advice, err := h.service.SubmitFeedback(c.Request().Context(), userID, c.Param("id"), service.FeedbackInput{
		IsHelpful: req.IsHelpful,
		Comment:   req.Comment,
	})
	if err != nil {
		return adviceToError(err)
	}
	return c.JSON(http.StatusOK, advice)
}
