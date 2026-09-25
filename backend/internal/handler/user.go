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
// プロフィール行が無い(初回アクセス)場合、サインアップ時にSupabase Authの
// user_metadataへ渡されたusernameがあればそれを使って行を作る(docs/adr/017)。
func (h *UserHandler) GetMe(c echo.Context) error {
	userID, _ := c.Get(appmw.ContextUserIDKey).(string)

	var username *string
	if v, _ := c.Get(appmw.ContextUsernameKey).(string); v != "" {
		username = &v
	}

	user, err := h.service.GetOrCreateMeWithUsername(c.Request().Context(), userID, username)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "プロフィールの取得に失敗しました")
	}
	return c.JSON(http.StatusOK, user)
}

type updateMeRequest struct {
	Region   *string `json:"region"`
	Username *string `json:"username"`
}

// UpdateMe は PUT /users/me。region(天候取得用の地域)・username(表示名)を更新する。
// 両方ともポインタで受け取り、リクエストに含まれなかったフィールド(nil)は変更しない
// (フェーズ7-2までのregion単体保存フォームと、フェーズ7-3で追加したusername保存フォームを
// 同じエンドポイントで共存させるため)。含まれているが空文字のフィールドは、
// その項目を明示的に未設定へ戻す(「(未設定)」を選び直して保存するケース)ものとして扱う。
func (h *UserHandler) UpdateMe(c echo.Context) error {
	userID, _ := c.Get(appmw.ContextUserIDKey).(string)

	var body updateMeRequest
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "リクエストボディが不正です")
	}
	if body.Region == nil && body.Username == nil {
		return echo.NewHTTPError(http.StatusBadRequest, "region かusernameのいずれかが必要です")
	}

	user, err := h.service.UpdateProfile(c.Request().Context(), userID, body.Region, body.Username)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "プロフィールの更新に失敗しました")
	}
	return c.JSON(http.StatusOK, user)
}

type changePasswordRequest struct {
	NewPassword string `json:"new_password"`
}

// ChangePassword は PUT /users/me/password。ログイン中ユーザーのパスワード変更(フェーズ7-4)。
// 「現在のパスワードが正しいか」はフロントエンドがSupabase Authへの再認証(signInWithPassword)で
// 事前に確認してから呼び出す前提(docs/adr/016)。ここでは新パスワードの設定のみ行う。
func (h *UserHandler) ChangePassword(c echo.Context) error {
	userID, _ := c.Get(appmw.ContextUserIDKey).(string)

	var body changePasswordRequest
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "リクエストボディが不正です")
	}
	if len(body.NewPassword) < 6 {
		return echo.NewHTTPError(http.StatusBadRequest, "パスワードは6文字以上で指定してください")
	}

	if err := h.service.UpdatePassword(c.Request().Context(), userID, body.NewPassword); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "パスワードの更新に失敗しました")
	}
	return c.NoContent(http.StatusNoContent)
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
