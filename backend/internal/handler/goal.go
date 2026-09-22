package handler

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	appmw "github.com/masa2050/pacely/backend/internal/middleware"
	"github.com/masa2050/pacely/backend/internal/model"
	"github.com/masa2050/pacely/backend/internal/repository"
	"github.com/masa2050/pacely/backend/internal/service"
)

type GoalHandler struct {
	service *service.GoalService
}

func NewGoalHandler(service *service.GoalService) *GoalHandler {
	return &GoalHandler{service: service}
}

// goalToError はservice層のエラーをdocs/api.md 4.のステータスコードに変換する。
func goalToError(err error) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "目標が見つかりません")
	case errors.Is(err, service.ErrForbidden):
		return echo.NewHTTPError(http.StatusForbidden, "他ユーザーの目標は操作できません")
	case errors.Is(err, service.ErrInvalidInput):
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, "処理に失敗しました")
	}
}

type goalRequest struct {
	GoalType      string     `json:"goal_type"`
	TargetTimeSec int        `json:"target_time_sec"`
	TargetDate    model.Date `json:"target_date"`
}

func (req goalRequest) toInput() service.GoalInput {
	return service.GoalInput{
		GoalType:      req.GoalType,
		TargetTimeSec: req.TargetTimeSec,
		TargetDate:    req.TargetDate,
	}
}

// Create は POST /goals。既存のactive目標はservice層で自動的にabandonedへ更新される。
func (h *GoalHandler) Create(c echo.Context) error {
	userID, _ := c.Get(appmw.ContextUserIDKey).(string)

	var req goalRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "リクエストボディが不正です")
	}

	goal, err := h.service.Create(c.Request().Context(), userID, req.toInput())
	if err != nil {
		return goalToError(err)
	}
	return c.JSON(http.StatusCreated, goal)
}

// List は GET /goals(履歴含む一覧)。
func (h *GoalHandler) List(c echo.Context) error {
	userID, _ := c.Get(appmw.ContextUserIDKey).(string)

	goals, err := h.service.List(c.Request().Context(), userID)
	if err != nil {
		return goalToError(err)
	}
	return c.JSON(http.StatusOK, goals)
}

// GetActive は GET /goals/active。有効な目標が無ければ404を返す。
func (h *GoalHandler) GetActive(c echo.Context) error {
	userID, _ := c.Get(appmw.ContextUserIDKey).(string)

	goal, err := h.service.GetActive(c.Request().Context(), userID)
	if err != nil {
		return goalToError(err)
	}
	return c.JSON(http.StatusOK, goal)
}

// GetProgress は GET /goals/active/progress。有効な目標が無ければ404を返す
// (GetActiveと同じくErrNotFound → 404、docs/api.md 2.4)。
func (h *GoalHandler) GetProgress(c echo.Context) error {
	userID, _ := c.Get(appmw.ContextUserIDKey).(string)

	progress, err := h.service.GetProgress(c.Request().Context(), userID)
	if err != nil {
		return goalToError(err)
	}
	return c.JSON(http.StatusOK, progress)
}

// Update は PUT /goals/{id}(目標タイム変更など)。
func (h *GoalHandler) Update(c echo.Context) error {
	userID, _ := c.Get(appmw.ContextUserIDKey).(string)

	var req goalRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "リクエストボディが不正です")
	}

	goal, err := h.service.Update(c.Request().Context(), userID, c.Param("id"), req.toInput())
	if err != nil {
		return goalToError(err)
	}
	return c.JSON(http.StatusOK, goal)
}

type goalStatusRequest struct {
	Status string `json:"status"`
}

// UpdateStatus は PATCH /goals/{id}/status(達成/断念のマーク付け)。
func (h *GoalHandler) UpdateStatus(c echo.Context) error {
	userID, _ := c.Get(appmw.ContextUserIDKey).(string)

	var req goalStatusRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "リクエストボディが不正です")
	}

	goal, err := h.service.UpdateStatus(c.Request().Context(), userID, c.Param("id"), req.Status)
	if err != nil {
		return goalToError(err)
	}
	return c.JSON(http.StatusOK, goal)
}
