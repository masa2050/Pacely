package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/masa2050/pacely/backend/internal/model"
)

type GoalRepository struct {
	pool *pgxpool.Pool
}

func NewGoalRepository(pool *pgxpool.Pool) *GoalRepository {
	return &GoalRepository{pool: pool}
}

const goalColumns = `id, user_id, goal_type, target_time_sec, target_date, status, created_at`

func scanGoal(row pgx.Row) (*model.Goal, error) {
	var g model.Goal
	if err := row.Scan(&g.ID, &g.UserID, &g.GoalType, &g.TargetTimeSec, &g.TargetDate, &g.Status, &g.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &g, nil
}

// CreateAsNewActive は「有効な目標は1ユーザー1件のみ」(docs/database.md 3.3)を
// トランザクション内で担保しながら新規目標を作成する。
// 既存のactive目標があれば、新規目標作成に伴い自動的にabandoned(断念)へ更新する。
// 「達成」扱いにしたい場合は、事前に PATCH /goals/{id}/status で明示的に
// achievedへ更新してから新規作成する運用とする(docs/adr/009参照)。
func (r *GoalRepository) CreateAsNewActive(ctx context.Context, goal *model.Goal) (*model.Goal, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`UPDATE goals SET status = $1 WHERE user_id = $2 AND status = $3`,
		model.GoalStatusAbandoned, goal.UserID, model.GoalStatusActive); err != nil {
		return nil, err
	}

	row := tx.QueryRow(ctx,
		`INSERT INTO goals (user_id, goal_type, target_time_sec, target_date, status)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING `+goalColumns,
		goal.UserID, goal.GoalType, goal.TargetTimeSec, goal.TargetDate, model.GoalStatusActive)
	created, err := scanGoal(row)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return created, nil
}

// ListByUser はuser_idで絞り込んだ目標一覧(履歴含む)を新しい順に返す。
func (r *GoalRepository) ListByUser(ctx context.Context, userID string) ([]model.Goal, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+goalColumns+` FROM goals WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	goals := []model.Goal{}
	for rows.Next() {
		var g model.Goal
		if err := rows.Scan(&g.ID, &g.UserID, &g.GoalType, &g.TargetTimeSec, &g.TargetDate, &g.Status, &g.CreatedAt); err != nil {
			return nil, err
		}
		goals = append(goals, g)
	}
	return goals, rows.Err()
}

// GetActiveByUser は現在有効な目標(status=active)を1件取得する。
// 存在しない場合は ErrNotFound を返す(GET /goals/active は404扱い)。
func (r *GoalRepository) GetActiveByUser(ctx context.Context, userID string) (*model.Goal, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+goalColumns+` FROM goals WHERE user_id = $1 AND status = $2`,
		userID, model.GoalStatusActive)
	return scanGoal(row)
}

// GetByID はowner(user_id)を問わずidだけで取得する。
// 「他ユーザーの目標か・存在しないか」の区別はservice層で行う(docs/api.md 403/404の使い分け)。
func (r *GoalRepository) GetByID(ctx context.Context, id string) (*model.Goal, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+goalColumns+` FROM goals WHERE id = $1`, id)
	return scanGoal(row)
}

// Update は目標タイム変更など、goal_type/target_time_sec/target_dateの編集用(PUT /goals/{id})。
// statusはUpdateStatusで別途更新する(責務を分ける)。
func (r *GoalRepository) Update(ctx context.Context, goal *model.Goal) (*model.Goal, error) {
	row := r.pool.QueryRow(ctx,
		`UPDATE goals SET goal_type = $1, target_time_sec = $2, target_date = $3
		 WHERE id = $4
		 RETURNING `+goalColumns,
		goal.GoalType, goal.TargetTimeSec, goal.TargetDate, goal.ID)
	return scanGoal(row)
}

// UpdateStatus は達成/断念のマーク付け用(PATCH /goals/{id}/status)。
func (r *GoalRepository) UpdateStatus(ctx context.Context, id, status string) (*model.Goal, error) {
	row := r.pool.QueryRow(ctx,
		`UPDATE goals SET status = $1 WHERE id = $2 RETURNING `+goalColumns,
		status, id)
	return scanGoal(row)
}
