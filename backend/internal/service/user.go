// Package service はビジネスロジックを担当する(3層構成の中間層)。
package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/masa2050/pacely/backend/internal/model"
	"github.com/masa2050/pacely/backend/internal/repository"
)

type UserService struct {
	repo             *repository.UserRepository
	supabaseURL      string
	supabaseAdminKey string
	httpClient       *http.Client
}

func NewUserService(repo *repository.UserRepository, supabaseURL string, supabaseServiceRoleKey string) *UserService {
	return &UserService{
		repo:             repo,
		supabaseURL:      supabaseURL,
		supabaseAdminKey: supabaseServiceRoleKey,
		httpClient:       &http.Client{Timeout: 10 * time.Second},
	}
}

// GetOrCreateMe は自分のプロフィールを取得する。
//
// Supabase Auth 側でユーザーが作られても、Pacely独自のusersテーブルの行は
// 自動では作られない(サインアップ時のWebhook等はMVPでは実装しない)。
// そのため「初回の /users/me アクセス時に行が無ければ作る」という
// get-or-create方式にして、フロント・バックエンドどちらにも
// 追加のサインアップ処理を増やさないようにしている。
func (s *UserService) GetOrCreateMe(ctx context.Context, userID string) (*model.User, error) {
	u, err := s.repo.GetByID(ctx, userID)
	if err == nil {
		return u, nil
	}
	if errors.Is(err, repository.ErrNotFound) {
		return s.repo.Create(ctx, userID)
	}
	return nil, err
}

func (s *UserService) UpdateRegion(ctx context.Context, userID string, region string) (*model.User, error) {
	return s.repo.UpdateRegion(ctx, userID, region)
}

// DeleteAccount は退会処理(docs/adr/014)。
//
// 1. Pacely独自データ(users行、およびON DELETE CASCADEで連動するruns/goals/advices)を削除
// 2. Supabase Auth側のログインアカウントも削除(管理者APIを使用)
//
// この順序にしているのは、途中で失敗した場合の状態を安全側に倒すため。
// 逆順(Auth削除→DB削除)だと、Auth削除後にDB削除が失敗すると、
// 二度とログインできない=二度と削除リクエストを送れないユーザーの
// データがDBに孤立して残ってしまう。DB削除を先にしておけば、
// 万が一Auth削除が失敗してもログインし直して再度削除を試せる。
func (s *UserService) DeleteAccount(ctx context.Context, userID string) error {
	if err := s.repo.Delete(ctx, userID); err != nil {
		return fmt.Errorf("ユーザーデータの削除に失敗: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete,
		s.supabaseURL+"/auth/v1/admin/users/"+userID, nil)
	if err != nil {
		return err
	}
	req.Header.Set("apikey", s.supabaseAdminKey)
	req.Header.Set("Authorization", "Bearer "+s.supabaseAdminKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("Supabase Authアカウントの削除に失敗: %w", err)
	}
	defer resp.Body.Close()

	// 404はすでに削除済み(再試行など)とみなし成功扱いにする。
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Supabase Authアカウントの削除に失敗(status %d): %s", resp.StatusCode, string(body))
	}
	return nil
}
