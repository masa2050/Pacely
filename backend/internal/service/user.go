// Package service はビジネスロジックを担当する(3層構成の中間層)。
package service

import (
	"bytes"
	"context"
	"encoding/json"
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
//
// runs/goals/advicesの作成時にも「プロフィール行の存在を保証する」目的で呼ばれるため
// (internal/service/run.go等)、usernameを持たないシグネチャのまま残す。
// usernameを渡したい場合はGetOrCreateMeWithUsernameを使う。
func (s *UserService) GetOrCreateMe(ctx context.Context, userID string) (*model.User, error) {
	return s.getOrCreate(ctx, userID, nil)
}

// GetOrCreateMeWithUsername はGetOrCreateMeと同じだが、行を新規作成する際に
// usernameも一緒に設定する(フェーズ7-3、docs/adr/017)。
// サインアップ時にsupabase.auth.signUpのoptions.dataへ渡したusernameは、JWTの
// user_metadataクレーム経由でミドルウェア(appmw.ContextUsernameKey)から取得できる。
// GET /users/meのハンドラからのみ呼ばれる想定。
func (s *UserService) GetOrCreateMeWithUsername(ctx context.Context, userID string, username *string) (*model.User, error) {
	return s.getOrCreate(ctx, userID, username)
}

func (s *UserService) getOrCreate(ctx context.Context, userID string, username *string) (*model.User, error) {
	u, err := s.repo.GetByID(ctx, userID)
	if err == nil {
		return u, nil
	}
	if errors.Is(err, repository.ErrNotFound) {
		return s.repo.Create(ctx, userID, username)
	}
	return nil, err
}

func (s *UserService) UpdateProfile(ctx context.Context, userID string, region, username *string) (*model.User, error) {
	return s.repo.UpdateProfile(ctx, userID, region, username)
}

// UpdatePassword はログイン中ユーザーのパスワードを変更する(フェーズ7-4、docs/adr/016)。
// 未ログイン時のメールリンク経由の再設定(docs/adr/010)とは別のフローで、
// こちらはSupabase Authの管理者APIをバックエンド経由で呼び出す。
// 「現在のパスワードが正しいか」の確認はフロントエンドがsignInWithPasswordで
// 再認証してから呼び出す前提とし、ここでは新しいパスワードの設定のみを行う
// (DeleteAccountと同じ管理者API呼び出しパターン)。
func (s *UserService) UpdatePassword(ctx context.Context, userID string, newPassword string) error {
	// %qはGo文字列のエスケープ規則でありJSONのエスケープ規則とは異なるため、
	// 制御文字を含むパスワードで不正なJSONになるのを避けるためjson.Marshalを使う。
	payload, err := json.Marshal(struct {
		Password string `json:"password"`
	}{Password: newPassword})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut,
		s.supabaseURL+"/auth/v1/admin/users/"+userID, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("apikey", s.supabaseAdminKey)
	req.Header.Set("Authorization", "Bearer "+s.supabaseAdminKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("パスワードの更新に失敗: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("パスワードの更新に失敗(status %d): %s", resp.StatusCode, string(body))
	}
	return nil
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
