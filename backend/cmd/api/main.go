package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
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

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("環境変数 DATABASE_URL が未設定です")
	}

	// 起動時にDBへ接続し、疎通確認(Ping)する。
	// ここで失敗したら早期に落として原因に気づけるようにする。
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
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

	// ヘルスチェック: サーバ生存 + DB疎通をまとめて確認できるようにする。
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

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("サーバ起動: http://localhost:%s", port)
	if err := e.Start(":" + port); err != nil && err != http.ErrServerClosed {
		log.Fatalf("サーバ起動に失敗: %v", err)
	}
}
