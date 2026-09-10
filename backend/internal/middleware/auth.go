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
// keyfn には JWKS(公開鍵)から署名検証用の鍵を返す関数を渡す(main で組み立てる)。
// Supabase の JWT は ES256(非対称鍵)で署名されているため、共有シークレットではなく
// 公開鍵で検証する。
//
// 検証内容:
//   - Authorization ヘッダーが "Bearer <token>" 形式か
//   - 署名アルゴリズムが ES256 か(アルゴリズム混同攻撃を防ぐため明示的に制限)
//   - JWKS の公開鍵による署名が正しいか
//   - 有効期限(exp)が切れていないか
//   - aud クレームが "authenticated" か(Supabase のログイン済みユーザー)
//
// 検証に成功したら sub クレーム(= Supabase の user id)をコンテキストに入れる。
func JWTAuth(keyfn jwt.Keyfunc) echo.MiddlewareFunc {
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{"ES256"}),
		jwt.WithAudience("authenticated"),
		jwt.WithExpirationRequired(),
	)

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

			claims := jwt.MapClaims{}
			token, err := parser.ParseWithClaims(tokenString, claims, keyfn)
			if err != nil || !token.Valid {
				return echo.NewHTTPError(http.StatusUnauthorized, "トークンが不正です")
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
