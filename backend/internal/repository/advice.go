package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/masa2050/pacely/backend/internal/model"
)

type AdviceRepository struct {
	pool *pgxpool.Pool
}

func NewAdviceRepository(pool *pgxpool.Pool) *AdviceRepository {
	return &AdviceRepository{pool: pool}
}

const adviceColumns = `id, user_id, goal_id, advice_text, next_menu, weather_context, generated_at`

func scanAdvice(row pgx.Row) (*model.Advice, error) {
	var a model.Advice
	if err := row.Scan(&a.ID, &a.UserID, &a.GoalID, &a.AdviceText, &a.NextMenu, &a.WeatherContext, &a.GeneratedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

// Create はAI生成結果を1件保存する(GET /advices/latestの生成フロー、docs/api.md 3.)。
func (r *AdviceRepository) Create(ctx context.Context, advice *model.Advice) (*model.Advice, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO advices (user_id, goal_id, advice_text, next_menu, weather_context)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING `+adviceColumns,
		advice.UserID, advice.GoalID, advice.AdviceText, advice.NextMenu, advice.WeatherContext)
	return scanAdvice(row)
}

// GetLatestByUser は最新のadviceを1件取得する(docs/database.md 3.4)。
// 存在しない場合はErrNotFoundを返す(「未生成」の判定に使う)。
func (r *AdviceRepository) GetLatestByUser(ctx context.Context, userID string) (*model.Advice, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+adviceColumns+` FROM advices WHERE user_id = $1 ORDER BY generated_at DESC LIMIT 1`,
		userID)
	return scanAdvice(row)
}

// ListByUser はuser_idで絞り込んだ提案履歴を新しい順に返す(GET /advices)。
func (r *AdviceRepository) ListByUser(ctx context.Context, userID string) ([]model.Advice, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+adviceColumns+` FROM advices WHERE user_id = $1 ORDER BY generated_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	advices := []model.Advice{}
	for rows.Next() {
		var a model.Advice
		if err := rows.Scan(&a.ID, &a.UserID, &a.GoalID, &a.AdviceText, &a.NextMenu, &a.WeatherContext, &a.GeneratedAt); err != nil {
			return nil, err
		}
		advices = append(advices, a)
	}
	return advices, rows.Err()
}
