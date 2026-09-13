// Package repository はDBアクセスのみを担当する(3層構成の最下層)。
// SQLはここに閉じ込め、service層より上にはSQLを書かない。
package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/masa2050/pacely/backend/internal/model"
)

// ErrNotFound はレコードが存在しない場合に返す。
// pgx.ErrNoRows をそのまま上位層に漏らさず、repository層で意味のあるエラーに変換する。
var ErrNotFound = errors.New("record not found")

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*model.User, error) {
	var u model.User
	row := r.pool.QueryRow(ctx, `SELECT id, region, created_at FROM users WHERE id = $1`, id)
	if err := row.Scan(&u.ID, &u.Region, &u.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

// Create はプロフィール行を新規作成する。region は未設定(null)で作る。
func (r *UserRepository) Create(ctx context.Context, id string) (*model.User, error) {
	var u model.User
	row := r.pool.QueryRow(ctx,
		`INSERT INTO users (id) VALUES ($1) RETURNING id, region, created_at`, id)
	if err := row.Scan(&u.ID, &u.Region, &u.CreatedAt); err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) UpdateRegion(ctx context.Context, id string, region string) (*model.User, error) {
	var u model.User
	row := r.pool.QueryRow(ctx,
		`UPDATE users SET region = $1 WHERE id = $2 RETURNING id, region, created_at`, region, id)
	if err := row.Scan(&u.ID, &u.Region, &u.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}
