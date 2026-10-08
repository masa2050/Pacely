package handler

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/masa2050/pacely/backend/internal/service"
)

func TestInvalidInputMessage(t *testing.T) {
	base := fmt.Errorf("%w: 距離は0より大きい数で入力してください", service.ErrInvalidInput)
	wrapped := fmt.Errorf("create run: %w", base)

	for name, err := range map[string]error{"直接": base, "ラップ済み": wrapped} {
		if got := invalidInputMessage(err); got != "距離は0より大きい数で入力してください" {
			t.Errorf("%s: got %q", name, got)
		}
	}
	if got := invalidInputMessage(service.ErrInvalidInput); strings.Contains(got, "invalid") {
		t.Errorf("理由が無い場合も英語を出さない: got %q", got)
	}
}

func TestHTTPErrorHandler(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = HTTPErrorHandler
	e.GET("/custom", func(c echo.Context) error {
		return echo.NewHTTPError(http.StatusBadRequest, "距離を入力してください")
	})
	e.GET("/plain", func(c echo.Context) error { return errors.New("pq: connection refused host=db.internal") })

	cases := []struct {
		name, method, path string
		wantCode           int
		wantBody           string
		notBody            string
	}{
		{"未定義ルート", http.MethodGet, "/nothing", 404, "リクエスト先が見つかりません", "Not Found"},
		{"メソッド違い", http.MethodPost, "/custom", 405, "この操作には対応していません", "Method Not Allowed"},
		{"日本語のmessageはそのまま", http.MethodGet, "/custom", 400, "距離を入力してください", ""},
		{"素のエラーは詳細を出さない", http.MethodGet, "/plain", 500, "サーバーでエラーが発生しました", "connection refused"},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
		if rec.Code != tc.wantCode || !strings.Contains(rec.Body.String(), tc.wantBody) {
			t.Errorf("%s: code=%d body=%s", tc.name, rec.Code, rec.Body.String())
		}
		if tc.notBody != "" && strings.Contains(rec.Body.String(), tc.notBody) {
			t.Errorf("%s: body に %q が含まれている: %s", tc.name, tc.notBody, rec.Body.String())
		}
	}
}

// echo.ErrNotFoundなどのグローバル変数は全リクエストで共有されるため、
// 日本語化のためにその場で書き換えていないこと(データ競合の防止)を確認する。
func TestHTTPErrorHandlerDoesNotMutateEchoGlobals(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = HTTPErrorHandler
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/nothing", nil))

	if echo.ErrNotFound.Message != http.StatusText(http.StatusNotFound) {
		t.Errorf("echo.ErrNotFound.Message が書き換わっている: %v", echo.ErrNotFound.Message)
	}
	if !strings.Contains(rec.Body.String(), "リクエスト先が見つかりません") {
		t.Errorf("レスポンスは日本語になっていない: %s", rec.Body.String())
	}
}

// InternalにHTTPErrorを持つエラーは、Echoの標準ハンドラが内側を返却対象にするため、
// 内側の英語messageも日本語化されることを確認する。
func TestHTTPErrorHandlerLocalizesInternalHTTPError(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = HTTPErrorHandler
	e.GET("/wrapped", func(c echo.Context) error {
		return echo.NewHTTPError(http.StatusInternalServerError, "x").SetInternal(echo.ErrNotFound)
	})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/wrapped", nil))

	if strings.Contains(rec.Body.String(), "Not Found") {
		t.Errorf("英語のmessageが残っている: %s", rec.Body.String())
	}
}
