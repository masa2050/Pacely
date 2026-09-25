// Package repository はDBアクセスのみを担当する(3層構成の最下層)。
// SQLはここに閉じ込め、service層より上にはSQLを書かない。
package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

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

// UpdateProfile はregion・usernameを更新する。どちらもポインタで、
// nil(リクエストにフィールドが無い)なら既存値のまま変更しない。
// ポインタが指す値が空文字の場合は、その項目をNULL(未設定)にクリアする
// (地域を「(未設定)」に戻せなかった不具合の修正、docs/database.md 3.1)。
func (r *UserRepository) UpdateProfile(ctx context.Context, id string, region, username *string) (*model.User, error) {
	sets := make([]string, 0, 2)
	args := make([]any, 0, 3)

	addField := func(column string, value *string) {
		if value == nil {
			return
		}
		args = append(args, nullIfEmpty(*value))
		sets = append(sets, fmt.Sprintf("%s = $%d", column, len(args)))
	}
	addField("region", region)
	addField("username", username)

	args = append(args, id)
	query := fmt.Sprintf(
		`UPDATE users SET %s WHERE id = $%d RETURNING id, region, username, created_at`,
		strings.Join(sets, ", "), len(args))

	var u model.User
	row := r.pool.QueryRow(ctx, query, args...)
	if err := row.Scan(&u.ID, &u.Region, &u.Username, &u.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
