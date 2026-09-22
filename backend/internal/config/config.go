// Package config はアプリ起動に必要な設定値(環境変数)を1か所にまとめる。
// 「どの環境変数が必須か」をここを見れば分かる状態にしておく。
package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	DatabaseURL    string
	Port           string
	SupabaseURL    string
	FrontendOrigin string
	// SupabaseServiceRoleKey は退会機能(DELETE /users/me)でSupabase Auth
	// 管理者APIを呼び出すために使う(docs/adr/014)。強い権限を持つキーのため
	// 他のキーと同様に必須環境変数として起動時チェックする。
	SupabaseServiceRoleKey string
	// GeminiAPIKey はAI提案生成に使う(docs/adr/002: 開発段階ではClaudeではなくGeminiを使う)。
	GeminiAPIKey string
	// OpenWeatherMapAPIKey は天候情報取得に使う。
	OpenWeatherMapAPIKey string
}

// Load は環境変数から設定を読み込む。必須項目が欠けていればエラーを返す。
func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:            os.Getenv("DATABASE_URL"),
		Port:                   os.Getenv("PORT"),
		SupabaseURL:            strings.TrimRight(os.Getenv("SUPABASE_URL"), "/"),
		FrontendOrigin:         os.Getenv("FRONTEND_ORIGIN"),
		SupabaseServiceRoleKey: os.Getenv("SUPABASE_SERVICE_ROLE_KEY"),
		GeminiAPIKey:           os.Getenv("GEMINI_API_KEY"),
		OpenWeatherMapAPIKey:   os.Getenv("OPENWEATHERMAP_API_KEY"),
	}

	var missing []string
	if cfg.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if cfg.SupabaseURL == "" {
		missing = append(missing, "SUPABASE_URL")
	}
	if cfg.SupabaseServiceRoleKey == "" {
		missing = append(missing, "SUPABASE_SERVICE_ROLE_KEY")
	}
	// AI提案(フェーズ5)はPacelyの中核機能(CLAUDE.md)のため、他の必須環境変数と
	// 同様に起動時チェックの対象にする。キー未設定のまま起動してGET /advices/latest
	// 呼び出し時に初めて500で気づく、という事態を避ける。
	if cfg.GeminiAPIKey == "" {
		missing = append(missing, "GEMINI_API_KEY")
	}
	if cfg.OpenWeatherMapAPIKey == "" {
		missing = append(missing, "OPENWEATHERMAP_API_KEY")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("必須の環境変数が未設定です: %v", missing)
	}

	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	if cfg.FrontendOrigin == "" {
		// ローカルのVite開発サーバのデフォルトポート。
		cfg.FrontendOrigin = "http://localhost:5173"
	}
	return cfg, nil
}

// JWKSURL は Supabase Auth の公開鍵(JWKS)エンドポイントURLを返す。
// Supabase の JWT は ES256(非対称鍵)で署名されており、この公開鍵で検証する。
func (c *Config) JWKSURL() string {
	return c.SupabaseURL + "/auth/v1/.well-known/jwks.json"
}
