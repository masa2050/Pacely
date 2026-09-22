package service

import (
	"context"
	"fmt"

	"github.com/masa2050/pacely/backend/internal/model"
	"github.com/masa2050/pacely/backend/internal/repository"
)

type GoalService struct {
	repo *repository.GoalRepository
	// runsと同様、目標作成前にusersテーブルの行を保証する(get-or-create)。
	// docs/adr/008-runs-user-fk-and-get-or-create.md のパターンを踏襲する。
	userService *UserService
	// GetProgress(直近の平均ペース)の計算にrunsが必要なため保持する。
	// 進捗計算はgoal+runsの両方を使うビジネスロジックであり、
	// docs/architecture.md 3.の方針通りservice層に置く。
	runRepo *repository.RunRepository
}

func NewGoalService(repo *repository.GoalRepository, userService *UserService, runRepo *repository.RunRepository) *GoalService {
	return &GoalService{repo: repo, userService: userService, runRepo: runRepo}
}

// goalTypeDistanceKm はGoalForm.tsxのGOAL_TYPESに対応する種目ごとの距離(km)。
// 目標ペース(target_time_sec / distance_km)の計算に使う。
var goalTypeDistanceKm = map[string]float64{
	"full_marathon": 42.195,
	"half_marathon": 21.0975,
	"10km":          10,
	"5km":           5,
}

// recentRunsForProgress は進捗算出に使う「直近の記録」の件数。
// runsテーブルの全件平均だと古い記録に引っ張られるため、直近のコンディションを
// 反映しやすい直近5件に絞る(advicesの「3件以上」判定とは別の閾値)。
const recentRunsForProgress = 5

// Progress は GET /goals/active/progress のレスポンス(docs/api.md 2.4)。
type Progress struct {
	Goal *model.Goal `json:"goal"`
	// TargetPaceSecPerKm は目標タイムから逆算した目標ペース(秒/km)。
	TargetPaceSecPerKm float64 `json:"target_pace_sec_per_km"`
	// RecentAveragePaceSecPerKm は直近記録の平均ペース。記録が1件も無ければnil。
	RecentAveragePaceSecPerKm *float64 `json:"recent_average_pace_sec_per_km"`
	RecentRunsCount           int      `json:"recent_runs_count"`
	// PaceDiffSecPerKm は 平均ペース-目標ペース。正なら目標より遅い、負なら速い。
	// 記録が1件も無ければnil。
	PaceDiffSecPerKm *float64 `json:"pace_diff_sec_per_km"`
}

// GoalInput はCreate/Updateで共通の入力値。
type GoalInput struct {
	GoalType      string
	TargetTimeSec int
	TargetDate    model.Date
}

func (in GoalInput) validate() error {
	if in.GoalType == "" {
		return fmt.Errorf("%w: goal_type は必須です", ErrInvalidInput)
	}
	if in.TargetTimeSec <= 0 {
		return fmt.Errorf("%w: target_time_sec は正の整数で指定してください", ErrInvalidInput)
	}
	if in.TargetDate.IsZero() {
		return fmt.Errorf("%w: target_date は必須です", ErrInvalidInput)
	}
	return nil
}

// updatableGoalStatuses はPATCH /goals/{id}/statusで受け付ける値。
// activeへの遷移はPOST /goals(新規作成)経由でのみ行う運用とし、
// ここでは「達成」「断念」の2つの終端状態のみ許可する。
var updatableGoalStatuses = map[string]bool{
	model.GoalStatusAchieved:  true,
	model.GoalStatusAbandoned: true,
}

func (s *GoalService) Create(ctx context.Context, userID string, in GoalInput) (*model.Goal, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}

	if _, err := s.userService.GetOrCreateMe(ctx, userID); err != nil {
		return nil, fmt.Errorf("ユーザープロフィールの確保に失敗: %w", err)
	}

	goal := &model.Goal{
		UserID:        userID,
		GoalType:      in.GoalType,
		TargetTimeSec: in.TargetTimeSec,
		TargetDate:    in.TargetDate,
	}
	return s.repo.CreateAsNewActive(ctx, goal)
}

func (s *GoalService) List(ctx context.Context, userID string) ([]model.Goal, error) {
	return s.repo.ListByUser(ctx, userID)
}

func (s *GoalService) GetActive(ctx context.Context, userID string) (*model.Goal, error) {
	return s.repo.GetActiveByUser(ctx, userID)
}

func (s *GoalService) Update(ctx context.Context, userID, id string, in GoalInput) (*model.Goal, error) {
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

	existing.GoalType = in.GoalType
	existing.TargetTimeSec = in.TargetTimeSec
	existing.TargetDate = in.TargetDate
	return s.repo.Update(ctx, existing)
}

// GetProgress は現在の目標に対する進捗サマリを返す(docs/api.md 2.4)。
// 有効な目標が無い場合はErrNotFound(GET /goals/activeと同じく404扱い)。
func (s *GoalService) GetProgress(ctx context.Context, userID string) (*Progress, error) {
	goal, err := s.repo.GetActiveByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	distanceKm, ok := goalTypeDistanceKm[goal.GoalType]
	if !ok {
		return nil, fmt.Errorf("%w: 未知のgoal_typeです: %s", ErrInvalidInput, goal.GoalType)
	}
	targetPace := float64(goal.TargetTimeSec) / distanceKm

	runs, err := s.runRepo.ListRecentByUser(ctx, userID, recentRunsForProgress)
	if err != nil {
		return nil, err
	}

	progress := &Progress{
		Goal:               goal,
		TargetPaceSecPerKm: targetPace,
		RecentRunsCount:    len(runs),
	}
	if len(runs) > 0 {
		var sum float64
		for _, run := range runs {
			sum += run.PaceSecPerKm
		}
		avg := sum / float64(len(runs))
		diff := avg - targetPace
		progress.RecentAveragePaceSecPerKm = &avg
		progress.PaceDiffSecPerKm = &diff
	}
	return progress, nil
}

func (s *GoalService) UpdateStatus(ctx context.Context, userID, id, status string) (*model.Goal, error) {
	if !updatableGoalStatuses[status] {
		return nil, fmt.Errorf("%w: status は achieved または abandoned を指定してください", ErrInvalidInput)
	}

	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.UserID != userID {
		return nil, ErrForbidden
	}

	return s.repo.UpdateStatus(ctx, id, status)
}
