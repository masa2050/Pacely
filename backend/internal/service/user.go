// Package service はビジネスロジックを担当する(3層構成の中間層)。
package service

import (
	"context"
	"errors"

	"github.com/masa2050/pacely/backend/internal/model"
	"github.com/masa2050/pacely/backend/internal/repository"
)

type UserService struct {
	repo *repository.UserRepository
}

func NewUserService(repo *repository.UserRepository) *UserService {
	return &UserService{repo: repo}
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
