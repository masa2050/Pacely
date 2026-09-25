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
	row := r.pool.QueryRow(ctx, `SELECT id, region, username, created_at FROM users WHERE id = $1`, id)
	if err := row.Scan(&u.ID, &u.Region, &u.Username, &u.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

// Create はプロフィール行を新規作成する。region は未設定(null)で作る。
// username はサインアップ時にSupabase Authのuser_metadataへ渡された値を受け取れるように
// 引数で受け取る(フェーズ7-3、docs/adr/017)。nilなら未設定(null)のまま作る。
func (r *UserRepository) Create(ctx context.Context, id string, username *string) (*model.User, error) {
	var u model.User
	row := r.pool.QueryRow(ctx,
		`INSERT INTO users (id, username) VALUES ($1, $2) RETURNING id, region, username, created_at`, id, username)
	if err := row.Scan(&u.ID, &u.Region, &u.Username, &u.CreatedAt); err != nil {
		return nil, err
	}
	return &u, nil
}

// Delete はusers行を削除する。runs/goals/advicesの関連行はDB側の
// ON DELETE CASCADE(migrations/000005)で自動的に削除される。
// 対象行が無くてもエラーにしない(退会APIの再試行に対して冪等にするため)。
func (r *UserRepository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	return err
}

// UpdateProfile はregion・usernameを更新する。どちらもポインタで、nilの場合は
// COALESCEで既存値のまま変更しない(PUT /users/meで片方だけ送るケースに対応するため)。
func (r *UserRepository) UpdateProfile(ctx context.Context, id string, region, username *string) (*model.User, error) {
	var u model.User
	row := r.pool.QueryRow(ctx,
		`UPDATE users SET region = COALESCE($1, region), username = COALESCE($2, username)
		 WHERE id = $3 RETURNING id, region, username, created_at`, region, username, id)
	if err := row.Scan(&u.ID, &u.Region, &u.Username, &u.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}
