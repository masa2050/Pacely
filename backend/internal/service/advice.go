package service

import (
	"context"
	"errors"
	"time"

	"github.com/masa2050/pacely/backend/internal/model"
	"github.com/masa2050/pacely/backend/internal/repository"
)

// docs/requirements.md 4.コールドスタート対応方針: 記録3件未満はAI呼び出しをせず
// ガイダンスのみ返す。
const minRunsForAdvice = 3

// adviceFreshWindow は「前回生成から24時間以内なら再生成しない」の閾値
// (docs/api.md 3.、docs/adr/004)。
const adviceFreshWindow = 24 * time.Hour

// recentRunsForAdvicePrompt はAIプロンプトに渡す「直近の記録」の件数。
// docs/adr/011で進捗計算に使った直近5件の考え方をそのまま再利用する
// (「直近の傾向」を渡すという目的が共通のため)。
const recentRunsForAdvicePrompt = recentRunsForProgress

// AdviceStatus は GET /advices/latest のレスポンスがどの状態かを表す
// (docs/api.md 3.の生成判定フローに対応)。
type AdviceStatus string

const (
	// AdviceStatusReady はadvice(生成済みまたはキャッシュ)を返せる状態。
	AdviceStatusReady AdviceStatus = "ready"
	// AdviceStatusNeedsMoreRuns は記録がminRunsForAdvice未満の状態。
	AdviceStatusNeedsMoreRuns AdviceStatus = "needs_more_runs"
	// AdviceStatusNeedsActiveGoal は有効な目標が無い状態。
	AdviceStatusNeedsActiveGoal AdviceStatus = "needs_active_goal"
)

// LatestAdviceResult は GET /advices/latest のレスポンス本体。
// AI呼び出しをスキップした場合(記録不足・目標未設定)もフロントが
// 分岐しやすいようStatusで明示する(docs/requirements.md 4.のガイダンス表示用)。
type LatestAdviceResult struct {
	Status          AdviceStatus  `json:"status"`
	Advice          *model.Advice `json:"advice,omitempty"`
	RunsCount       int           `json:"runs_count"`
	MinRunsRequired int           `json:"min_runs_required"`
}

type AdviceService struct {
	repo        *repository.AdviceRepository
	userService *UserService
	runRepo     *repository.RunRepository
	goalRepo    *repository.GoalRepository
	weather     WeatherClient
	ai          AdviceGenerator
}

func NewAdviceService(
	repo *repository.AdviceRepository,
	userService *UserService,
	runRepo *repository.RunRepository,
	goalRepo *repository.GoalRepository,
	weather WeatherClient,
	ai AdviceGenerator,
) *AdviceService {
	return &AdviceService{repo: repo, userService: userService, runRepo: runRepo, goalRepo: goalRepo, weather: weather, ai: ai}
}

// GetLatest は GET /advices/latest の生成判定・生成処理本体(docs/api.md 3.)。
// GETだが意図的に副作用(AI呼び出し・DB書き込み)を持つ設計であり、これは
// docs/adr/004で明記済みの意図的な判断(CLAUDE.mdのルール5)。
func (s *AdviceService) GetLatest(ctx context.Context, userID string) (*LatestAdviceResult, error) {
	runsCount, err := s.runRepo.CountByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if runsCount < minRunsForAdvice {
		return &LatestAdviceResult{Status: AdviceStatusNeedsMoreRuns, RunsCount: runsCount, MinRunsRequired: minRunsForAdvice}, nil
	}

	goal, err := s.goalRepo.GetActiveByUser(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return &LatestAdviceResult{Status: AdviceStatusNeedsActiveGoal, RunsCount: runsCount, MinRunsRequired: minRunsForAdvice}, nil
		}
		return nil, err
	}

	latest, err := s.repo.GetLatestByUser(ctx, userID)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	needsGenerate := errors.Is(err, repository.ErrNotFound) || time.Since(latest.GeneratedAt) >= adviceFreshWindow

	if !needsGenerate {
		return &LatestAdviceResult{Status: AdviceStatusReady, Advice: latest, RunsCount: runsCount, MinRunsRequired: minRunsForAdvice}, nil
	}

	generated, err := s.generate(ctx, userID, goal)
	if err != nil {
		return nil, err
	}
	return &LatestAdviceResult{Status: AdviceStatusReady, Advice: generated, RunsCount: runsCount, MinRunsRequired: minRunsForAdvice}, nil
}

// generate は天候取得→AI呼び出し→DB保存までを実行する(docs/api.md 3.のa〜d)。
func (s *AdviceService) generate(ctx context.Context, userID string, goal *model.Goal) (*model.Advice, error) {
	user, err := s.userService.GetOrCreateMe(ctx, userID)
	if err != nil {
		return nil, err
	}

	recentRuns, err := s.runRepo.ListRecentByUser(ctx, userID, recentRunsForAdvicePrompt)
	if err != nil {
		return nil, err
	}

	// regionが未設定の場合は天候APIを呼ばずに生成する(docs/adr/012)。
	// 「地域未設定」は入力エラーではなく想定内の状態であり、AI提案自体を
	// ブロックする理由にはならないため、天候情報なしのまま続行する。
	var weather model.WeatherContext
	if user.Region != nil && *user.Region != "" {
		weather, err = s.weather.CurrentWeather(ctx, *user.Region)
		if err != nil {
			return nil, err
		}
	}

	generation, err := s.ai.GenerateAdvice(ctx, AdvicePromptInput{
		GoalType:      goal.GoalType,
		TargetTimeSec: goal.TargetTimeSec,
		TargetDate:    goal.TargetDate,
		RecentRuns:    recentRuns,
		Weather:       weather,
	})
	if err != nil {
		return nil, err
	}

	advice := &model.Advice{
		UserID:         userID,
		GoalID:         &goal.ID,
		AdviceText:     generation.AdviceText,
		NextMenu:       generation.NextMenu,
		WeatherContext: weather,
	}
	return s.repo.Create(ctx, advice)
}

// List は GET /advices(過去の提案履歴一覧)。
func (s *AdviceService) List(ctx context.Context, userID string) ([]model.Advice, error) {
	return s.repo.ListByUser(ctx, userID)
}
