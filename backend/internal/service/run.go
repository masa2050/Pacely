package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/masa2050/pacely/backend/internal/model"
	"github.com/masa2050/pacely/backend/internal/repository"
)

// ErrForbidden は他ユーザーの記録を操作しようとした場合に返す。
// docs/api.md: 404(存在しない)と403(自分のものではない)を区別するため、
// repositoryでは所有者を問わず取得し、所有者チェックはここで行う。
var ErrForbidden = errors.New("forbidden")

// ErrInvalidInput は入力値がドメインルールを満たさない場合に返す(400扱い)。
var ErrInvalidInput = errors.New("invalid input")

type RunService struct {
	repo *repository.RunRepository
	// 記録作成前にusersテーブルの行を保証する(get-or-create)ために使う。
	// usersはADR 007の方針で「初回 GET/PUT /users/me」まで行が作られないため、
	// runs.user_idのFK制約を満たすにはここで先に確保しておく必要がある。
	userService *UserService
}

func NewRunService(repo *repository.RunRepository, userService *UserService) *RunService {
	return &RunService{repo: repo, userService: userService}
}

// RunInput はCreate/Updateで共通の入力値。
type RunInput struct {
	DistanceKm  float64
	DurationSec int
	RPE         *int
	RunDate     model.Date
}

func (in RunInput) validate() error {
	if in.DistanceKm <= 0 {
		return fmt.Errorf("%w: distance_km は正の数で指定してください", ErrInvalidInput)
	}
	if in.DurationSec <= 0 {
		return fmt.Errorf("%w: duration_sec は正の整数で指定してください", ErrInvalidInput)
	}
	if in.RunDate.IsZero() {
		return fmt.Errorf("%w: run_date は必須です", ErrInvalidInput)
	}
	if in.RPE != nil && (*in.RPE < 1 || *in.RPE > 10) {
		return fmt.Errorf("%w: rpe は1〜10で指定してください(docs/adr/003)", ErrInvalidInput)
	}
	return nil
}

// pace は秒/kmで計算する。distance_kmは保存時に計算し直さないよう、
// runsテーブルに保存しておく(docs/database.md 5.)。
func pace(distanceKm float64, durationSec int) float64 {
	return float64(durationSec) / distanceKm
}

func (s *RunService) Create(ctx context.Context, userID string, in RunInput) (*model.Run, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}

	if _, err := s.userService.GetOrCreateMe(ctx, userID); err != nil {
		return nil, fmt.Errorf("ユーザープロフィールの確保に失敗: %w", err)
	}

	run := &model.Run{
		UserID:       userID,
		DistanceKm:   in.DistanceKm,
		DurationSec:  in.DurationSec,
		PaceSecPerKm: pace(in.DistanceKm, in.DurationSec),
		RPE:          in.RPE,
		RunDate:      in.RunDate,
	}
	return s.repo.Create(ctx, run)
}

func (s *RunService) List(ctx context.Context, userID string, from, to *time.Time) ([]model.Run, error) {
	return s.repo.ListByUser(ctx, userID, from, to)
}

func (s *RunService) Get(ctx context.Context, userID, id string) (*model.Run, error) {
	run, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if run.UserID != userID {
		return nil, ErrForbidden
	}
	return run, nil
}

func (s *RunService) Update(ctx context.Context, userID, id string, in RunInput) (*model.Run, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}

	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.UserID != userID {
		return nil, ErrForbidden
	}

	existing.DistanceKm = in.DistanceKm
	existing.DurationSec = in.DurationSec
	existing.PaceSecPerKm = pace(in.DistanceKm, in.DurationSec)
	existing.RPE = in.RPE
	existing.RunDate = in.RunDate
	return s.repo.Update(ctx, existing)
}

func (s *RunService) Delete(ctx context.Context, userID, id string) error {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if existing.UserID != userID {
		return ErrForbidden
	}
	return s.repo.Delete(ctx, id)
}
