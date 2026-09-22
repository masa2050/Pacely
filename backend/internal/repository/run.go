package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/masa2050/pacely/backend/internal/model"
)

type RunRepository struct {
	pool *pgxpool.Pool
}

func NewRunRepository(pool *pgxpool.Pool) *RunRepository {
	return &RunRepository{pool: pool}
}

const runColumns = `id, user_id, distance_km, duration_sec, pace_sec_per_km, rpe, run_date, created_at`

func scanRun(row pgx.Row) (*model.Run, error) {
	var r model.Run
	if err := row.Scan(&r.ID, &r.UserID, &r.DistanceKm, &r.DurationSec, &r.PaceSecPerKm, &r.RPE, &r.RunDate, &r.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &r, nil
}

func (r *RunRepository) Create(ctx context.Context, run *model.Run) (*model.Run, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO runs (user_id, distance_km, duration_sec, pace_sec_per_km, rpe, run_date)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING `+runColumns,
		run.UserID, run.DistanceKm, run.DurationSec, run.PaceSecPerKm, run.RPE, run.RunDate)
	return scanRun(row)
}

// ListByUser はuser_idで絞り込んだ記録一覧を新しい順に返す。
// from/toはrun_dateの範囲指定(どちらもnilなら絞り込みなし)。
// pgxの引数エンコードが確実にサポートするtime.Timeで受け取る(model.Dateではなく)。
func (r *RunRepository) ListByUser(ctx context.Context, userID string, from, to *time.Time) ([]model.Run, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+runColumns+`
		 FROM runs
		 WHERE user_id = $1
		   AND ($2::date IS NULL OR run_date >= $2)
		   AND ($3::date IS NULL OR run_date <= $3)
		 ORDER BY run_date DESC, created_at DESC`,
		userID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	runs := []model.Run{}
	for rows.Next() {
		var run model.Run
		if err := rows.Scan(&run.ID, &run.UserID, &run.DistanceKm, &run.DurationSec, &run.PaceSecPerKm, &run.RPE, &run.RunDate, &run.CreatedAt); err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

// ListRecentByUser は直近limit件の記録を新しい順に返す(進捗計算用)。
// docs/api.md 2.4 GET /goals/active/progress で「直近の平均ペース」を出すために使う。
func (r *RunRepository) ListRecentByUser(ctx context.Context, userID string, limit int) ([]model.Run, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+runColumns+`
		 FROM runs
		 WHERE user_id = $1
		 ORDER BY run_date DESC, created_at DESC
		 LIMIT $2`,
		userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	runs := []model.Run{}
	for rows.Next() {
		var run model.Run
		if err := rows.Scan(&run.ID, &run.UserID, &run.DistanceKm, &run.DurationSec, &run.PaceSecPerKm, &run.RPE, &run.RunDate, &run.CreatedAt); err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

// CountByUser はuser_idの記録件数を返す。
// GET /advices/latest の「3件未満はガイダンスのみ返す」判定に使う
// (docs/requirements.md 4.コールドスタート対応方針)。一覧を全件取得して
// len()を数えるより、COUNTをDBに任せた方が記録数が増えても軽い。
func (r *RunRepository) CountByUser(ctx context.Context, userID string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM runs WHERE user_id = $1`, userID).Scan(&count)
	return count, err
}

// GetByID はowner(user_id)を問わずidだけで取得する。
// 「他ユーザーの記録か・存在しないか」の区別はservice層で行う(docs/api.md 403/404の使い分け)。
func (r *RunRepository) GetByID(ctx context.Context, id string) (*model.Run, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+runColumns+` FROM runs WHERE id = $1`, id)
	return scanRun(row)
}

func (r *RunRepository) Update(ctx context.Context, run *model.Run) (*model.Run, error) {
	row := r.pool.QueryRow(ctx,
		`UPDATE runs
		 SET distance_km = $1, duration_sec = $2, pace_sec_per_km = $3, rpe = $4, run_date = $5
		 WHERE id = $6
		 RETURNING `+runColumns,
		run.DistanceKm, run.DurationSec, run.PaceSecPerKm, run.RPE, run.RunDate, run.ID)
	return scanRun(row)
}

func (r *RunRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM runs WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
