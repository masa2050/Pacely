package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/masa2050/pacely/backend/internal/config"
	"github.com/masa2050/pacely/backend/internal/handler"
	appmw "github.com/masa2050/pacely/backend/internal/middleware"
	"github.com/masa2050/pacely/backend/internal/repository"
	"github.com/masa2050/pacely/backend/internal/service"
)

func main() {
	// ローカル開発では .env を読む。本番(Railway等)は環境変数が直接渡るので、
	// .env が無くてもエラーにはしない。
	// candidates を1つずつ試す(godotenv.Load は複数指定すると最初の失敗で止まるため)。
	for _, path := range []string{".env", "../.env"} {
		if err := godotenv.Load(path); err == nil {
			log.Printf(".env を読み込みました: %s", path)
			break
		}
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	// 起動時にDBへ接続し、疎通確認(Ping)する。
	// ここで失敗したら早期に落として原因に気づけるようにする。
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("DB接続プールの作成に失敗: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("DBへのPingに失敗: %v", err)
	}
	log.Println("DB接続OK")

	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	// フロントエンド(Vite開発サーバ/本番Vercel)からのAuthorizationヘッダー付き
	// リクエストを許可する。ブラウザはこのヘッダーがあるとpreflight(OPTIONS)を
	// 送るため、CORS設定が無いと "/users/me" などが全滅する。
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{cfg.FrontendOrigin},
		AllowHeaders: []string{echo.HeaderAuthorization, echo.HeaderContentType},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete},
	}))

	// ヘルスチェック: 認証不要。サーバ生存 + DB疎通をまとめて確認できるようにする。
	e.GET("/health", func(c echo.Context) error {
		pingCtx, pingCancel := context.WithTimeout(c.Request().Context(), 2*time.Second)
		defer pingCancel()

		if err := pool.Ping(pingCtx); err != nil {
			return c.JSON(http.StatusServiceUnavailable, echo.Map{
				"status": "degraded",
				"db":     "down",
			})
		}
		return c.JSON(http.StatusOK, echo.Map{
			"status": "ok",
			"db":     "up",
		})
	})

	// Supabase Auth の公開鍵(JWKS)を取得する。取得後はライブラリが定期的に
	// 鍵をリフレッシュする(鍵ローテーションに追従するため)。
	jwks, err := keyfunc.NewDefault([]string{cfg.JWKSURL()})
	if err != nil {
		log.Fatalf("JWKS の取得に失敗: %v", err)
	}
	log.Println("JWKS 取得OK")

	// handler → service → repository の3層構成(docs/architecture.md)。
	userRepo := repository.NewUserRepository(pool)
	// 退会機能(DELETE /users/me)でSupabase Auth管理者APIを呼ぶため、
	// SupabaseURL・ServiceRoleKeyを注入する(docs/adr/014)。
	userService := service.NewUserService(userRepo, cfg.SupabaseURL, cfg.SupabaseServiceRoleKey)
	userHandler := handler.NewUserHandler(userService)

	runRepo := repository.NewRunRepository(pool)
	// RunServiceはusersテーブルのget-or-createにUserServiceを使う
	// (docs/adr/007-users-profile-sync-strategy.md)。
	runService := service.NewRunService(runRepo, userService)
	runHandler := handler.NewRunHandler(runService)

	goalRepo := repository.NewGoalRepository(pool)
	// GoalServiceもrunsと同じget-or-createパターンを踏襲する(docs/adr/008)。
	// 進捗(フェーズ4)の平均ペース算出にrunRepoも必要なため合わせて渡す。
	goalService := service.NewGoalService(goalRepo, userService, runRepo)
	goalHandler := handler.NewGoalHandler(goalService)

	adviceRepo := repository.NewAdviceRepository(pool)
	// 天候・AI呼び出しはインターフェース越しに注入する(docs/architecture.md 7.)。
	// 開発段階のAI実装はGemini(docs/adr/002)。
	weatherClient := service.NewOpenWeatherMapClient(cfg.OpenWeatherMapAPIKey)
	aiClient := service.NewGeminiClient(cfg.GeminiAPIKey)
	adviceService := service.NewAdviceService(adviceRepo, userService, runRepo, goalRepo, weatherClient, aiClient)
	adviceHandler := handler.NewAdviceHandler(adviceService)

	// 認証が必要なルートは api グループにまとめる。
	api := e.Group("")
	api.Use(appmw.JWTAuth(jwks.Keyfunc))
	api.GET("/users/me", userHandler.GetMe)
	api.PUT("/users/me", userHandler.UpdateMe)
	api.PUT("/users/me/password", userHandler.ChangePassword)
	api.DELETE("/users/me", userHandler.DeleteMe)

	api.POST("/runs", runHandler.Create)
	api.GET("/runs", runHandler.List)
	api.GET("/runs/:id", runHandler.Get)
	api.PUT("/runs/:id", runHandler.Update)
	api.DELETE("/runs/:id", runHandler.Delete)

	api.POST("/goals", goalHandler.Create)
	api.GET("/goals", goalHandler.List)
	api.GET("/goals/active", goalHandler.GetActive)
	api.GET("/goals/active/progress", goalHandler.GetProgress)
	api.PUT("/goals/:id", goalHandler.Update)
	api.PATCH("/goals/:id/status", goalHandler.UpdateStatus)

	api.GET("/advices/latest", adviceHandler.GetLatest)
	api.GET("/advices", adviceHandler.List)

	log.Printf("サーバ起動: http://localhost:%s", cfg.Port)
	if err := e.Start(":" + cfg.Port); err != nil && err != http.ErrServerClosed {
		log.Fatalf("サーバ起動に失敗: %v", err)
	}
}
