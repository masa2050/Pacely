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
}

// Load は環境変数から設定を読み込む。必須項目が欠けていればエラーを返す。
func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		Port:           os.Getenv("PORT"),
		SupabaseURL:    strings.TrimRight(os.Getenv("SUPABASE_URL"), "/"),
		FrontendOrigin: os.Getenv("FRONTEND_ORIGIN"),
	}

	var missing []string
	if cfg.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if cfg.SupabaseURL == "" {
		missing = append(missing, "SUPABASE_URL")
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
