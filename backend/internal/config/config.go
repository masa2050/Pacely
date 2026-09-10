// Package config はアプリ起動に必要な設定値(環境変数)を1か所にまとめる。
// 「どの環境変数が必須か」をここを見れば分かる状態にしておく。
package config

import (
	"fmt"
	"os"
)

type Config struct {
	DatabaseURL      string
	Port             string
	SupabaseJWTSecret string
}

// Load は環境変数から設定を読み込む。必須項目が欠けていればエラーを返す。
func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		Port:              os.Getenv("PORT"),
		SupabaseJWTSecret: os.Getenv("SUPABASE_JWT_SECRET"),
	}

	var missing []string
	if cfg.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if cfg.SupabaseJWTSecret == "" {
		missing = append(missing, "SUPABASE_JWT_SECRET")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("必須の環境変数が未設定です: %v", missing)
	}

	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	return cfg, nil
}
