package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	appmw "github.com/masa2050/pacely/backend/internal/middleware"
	"github.com/masa2050/pacely/backend/internal/model"
	"github.com/masa2050/pacely/backend/internal/repository"
	"github.com/masa2050/pacely/backend/internal/service"
)

type RunHandler struct {
	service *service.RunService
}

func NewRunHandler(service *service.RunService) *RunHandler {
	return &RunHandler{service: service}
}

// runToError はservice層のエラーをdocs/api.md 4.のステータスコードに変換する。
func runToError(err error) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "記録が見つかりません")
	case errors.Is(err, service.ErrForbidden):
		return echo.NewHTTPError(http.StatusForbidden, "他ユーザーの記録は操作できません")
	case errors.Is(err, service.ErrInvalidInput):
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, "処理に失敗しました")
	}
}

type runRequest struct {
	DistanceKm  float64    `json:"distance_km"`
	DurationSec int        `json:"duration_sec"`
	RPE         *int       `json:"rpe"`
	RunDate     model.Date `json:"run_date"`
}

func (req runRequest) toInput() service.RunInput {
	return service.RunInput{
		DistanceKm:  req.DistanceKm,
		DurationSec: req.DurationSec,
		RPE:         req.RPE,
		RunDate:     req.RunDate,
	}
}

// Create は POST /runs。
func (h *RunHandler) Create(c echo.Context) error {
	userID, _ := c.Get(appmw.ContextUserIDKey).(string)

	var req runRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "リクエストボディが不正です")
	}

	run, err := h.service.Create(c.Request().Context(), userID, req.toInput())
	if err != nil {
		return runToError(err)
	}
	return c.JSON(http.StatusCreated, run)
}

// List は GET /runs?from=&to= 。
func (h *RunHandler) List(c echo.Context) error {
	userID, _ := c.Get(appmw.ContextUserIDKey).(string)

	from, err := parseDateParam(c.QueryParam("from"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "from は YYYY-MM-DD 形式で指定してください")
	}
	to, err := parseDateParam(c.QueryParam("to"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "to は YYYY-MM-DD 形式で指定してください")
	}

	runs, err := h.service.List(c.Request().Context(), userID, from, to)
	if err != nil {
		return runToError(err)
	}
	return c.JSON(http.StatusOK, runs)
}

func parseDateParam(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// Get は GET /runs/{id}。
func (h *RunHandler) Get(c echo.Context) error {
	userID, _ := c.Get(appmw.ContextUserIDKey).(string)

	run, err := h.service.Get(c.Request().Context(), userID, c.Param("id"))
	if err != nil {
		return runToError(err)
	}
	return c.JSON(http.StatusOK, run)
}

// Update は PUT /runs/{id}。
func (h *RunHandler) Update(c echo.Context) error {
	userID, _ := c.Get(appmw.ContextUserIDKey).(string)

	var req runRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "リクエストボディが不正です")
	}

	run, err := h.service.Update(c.Request().Context(), userID, c.Param("id"), req.toInput())
	if err != nil {
		return runToError(err)
	}
	return c.JSON(http.StatusOK, run)
}

// Delete は DELETE /runs/{id}。
func (h *RunHandler) Delete(c echo.Context) error {
	userID, _ := c.Get(appmw.ContextUserIDKey).(string)

	if err := h.service.Delete(c.Request().Context(), userID, c.Param("id")); err != nil {
		return runToError(err)
	}
	return c.NoContent(http.StatusNoContent)
}
