// Package middleware は Echo 用のカスタムミドルウェアを置く。
package middleware

import (
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

// コンテキストに user_id を格納するときのキー。
// ハンドラ側では c.Get(ContextUserIDKey) で取り出す。
const ContextUserIDKey = "user_id"

// JWTAuth は Supabase Auth が発行した JWT を検証するミドルウェアを返す。
//
// 検証内容:
//   - Authorization ヘッダーが "Bearer <token>" 形式か
//   - 署名アルゴリズムが HS256 か(アルゴリズム混同攻撃を防ぐため明示的にチェック)
//   - Supabase の JWT Secret による署名が正しいか
//   - 有効期限(exp)が切れていないか ... jwt ライブラリが自動で検証
//
// 検証に成功したら sub クレーム(= Supabase の user id)をコンテキストに入れる。
func JWTAuth(jwtSecret string) echo.MiddlewareFunc {
	secret := []byte(jwtSecret)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "Authorization ヘッダーがありません")
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				return echo.NewHTTPError(http.StatusUnauthorized, "Authorization ヘッダーの形式が不正です")
			}
			tokenString := parts[1]

			token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
				// アルゴリズム混同攻撃対策: HS256 以外は拒否する
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, echo.NewHTTPError(http.StatusUnauthorized, "想定外の署名アルゴリズムです")
				}
				return secret, nil
			})
			if err != nil || !token.Valid {
				return echo.NewHTTPError(http.StatusUnauthorized, "トークンが不正です")
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				return echo.NewHTTPError(http.StatusUnauthorized, "トークンのクレームを読めません")
			}

			sub, _ := claims["sub"].(string)
			if sub == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "トークンに sub(user id)がありません")
			}

			c.Set(ContextUserIDKey, sub)
			return next(c)
		}
	}
}
