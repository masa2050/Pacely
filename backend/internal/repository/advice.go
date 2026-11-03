package repository

import (
	"context"
	"errors"
	"time"

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

const adviceColumns = `id, user_id, goal_id, advice_text, next_menu, weather_context, generated_at,
	is_helpful, feedback_comment, feedback_at`

// scanAdvice は1行分をmodel.Adviceに読み出す。pgx.Rowsも pgx.Row(Scanのみ)を
// 満たすため、一覧取得のループからも同じ関数を使い列の並びを1か所にまとめている。
func scanAdvice(row pgx.Row) (*model.Advice, error) {
	var a model.Advice
	if err := row.Scan(&a.ID, &a.UserID, &a.GoalID, &a.AdviceText, &a.NextMenu, &a.WeatherContext, &a.GeneratedAt,
		&a.IsHelpful, &a.FeedbackComment, &a.FeedbackAt); err != nil {
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

// TryAcquireGenerationLease は「今からleaseの間、このユーザーのAI提案を生成する権利」を
// 取りにいく(docs/adr/031)。取得できたらtrue、他のリクエストが生成中ならfalse。
// 「空いているか確認する」と「予約を書き込む」を1文のUPDATEで同時に行うのが要点で、
// 2つのリクエストが同時に来ても、PostgreSQLが行ロックで順番に処理し、先に来た方だけが
// 1行更新に成功する。確認と書き込みを別のSQLに分けると、その隙間に割り込まれて
// 二重に取得できてしまう(check-then-act競合)。
// 権利は期限付きなので、生成の途中でサーバーが落ちても、期限が過ぎれば別のリクエストが取れる。
//
// 取得できた場合は、その権利を表すGenerationLeaseを返す。他のリクエストが生成中なら(nil, nil)。
// 「他のリクエストが生成中」と「usersの行が無い」はどちらも更新が0行になり区別できないため、
// 呼び出し側は先にusersの行の存在を保証しておくこと。
func (r *AdviceRepository) TryAcquireGenerationLease(ctx context.Context, userID string, lease time.Duration) (*GenerationLease, error) {
	var until time.Time
	err := r.pool.QueryRow(ctx,
		`UPDATE users
		 SET advice_lock_until = now() + make_interval(secs => $2)
		 WHERE id = $1 AND (advice_lock_until IS NULL OR advice_lock_until < now())
		 RETURNING advice_lock_until`,
		userID, lease.Seconds()).Scan(&until)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &GenerationLease{userID: userID, until: until}, nil
}

// GenerationLease は取得済みの生成の権利。取得時に書き込んだ期限を持ち、返すときの本人確認に使う。
type GenerationLease struct {
	userID string
	until  time.Time
}

// ReleaseGenerationLease は生成の権利を返す(期限を待たずに、次のリクエストが取れるようにする)。
// 「自分が取ったときの期限と一致する場合だけ」消す。生成が長引いて期限が切れ、別のリクエストが
// 新しく権利を取っている場合に、その人の権利まで消してしまわないため。
func (r *AdviceRepository) ReleaseGenerationLease(ctx context.Context, lease *GenerationLease) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET advice_lock_until = NULL WHERE id = $1 AND advice_lock_until = $2`,
		lease.userID, lease.until)
	return err
}

// GetLatestByUser は最新のadviceを1件取得する(docs/database.md 3.4)。
// 存在しない場合はErrNotFoundを返す(「未生成」の判定に使う)。
func (r *AdviceRepository) GetLatestByUser(ctx context.Context, userID string) (*model.Advice, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+adviceColumns+` FROM advices WHERE user_id = $1 ORDER BY generated_at DESC LIMIT 1`,
		userID)
	return scanAdvice(row)
}

// CountGeneratedSince は、直近window以内に生成された提案の件数を返す(docs/adr/034)。
// 目標の切り替えによる再生成に、1ユーザーあたりの回数上限をかけるために使う。
// 「直近」の基準はアプリのサーバーではなくDBのnow()に揃える(generated_atもDBのnow()で入るため)。
func (r *AdviceRepository) CountGeneratedSince(ctx context.Context, userID string, window time.Duration) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM advices WHERE user_id = $1 AND generated_at > now() - make_interval(secs => $2)`,
		userID, window.Seconds()).Scan(&n)
	return n, err
}

// scanAdvices はrows.Next()のループを1か所にまとめ、ListByUser/ListRecentByUserで
// 重複させない(/code-review指摘)。
func scanAdvices(rows pgx.Rows) ([]model.Advice, error) {
	advices := []model.Advice{}
	for rows.Next() {
		a, err := scanAdvice(rows)
		if err != nil {
			return nil, err
		}
		advices = append(advices, *a)
	}
	return advices, rows.Err()
}

// ListByUser はuser_idで絞り込んだ提案履歴を新しい順にlimit件返す(GET /advices)。
// 提案は1日1件生成されるため履歴は増え続ける。上限が無いと1年で365件・本文付きを
// 毎回返すことになるため、直近分のみに絞る(docs/adr/020)。上限値そのものは
// 「どこまで遡れれば足りるか」というビジネス上の判断なのでservice層に置き、
// repositoryは受け取った値をそのまま使う。
func (r *AdviceRepository) ListByUser(ctx context.Context, userID string, limit int) ([]model.Advice, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+adviceColumns+` FROM advices WHERE user_id = $1 ORDER BY generated_at DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAdvices(rows)
}

// ListRecentByGoal は同じgoal_idに絞り込んだ直近limit件の提案を新しい順に返す
// (docs/adr/021、8-2①)。AIへのプロンプトに「前回までに何を提案したか」を渡し、
// 同じメニューへの収束を避けるために使う。goal_idで絞るのは、目標を切り替えた
// 直後に旧目標向けの提案履歴が新しい目標の提案に混ざらないようにするため
// (/code-review指摘: 5km目標→フルマラソン目標に切り替えた直後、旧目標の
// 短い距離のメニューが「前回と違う刺激を」の基準に混ざるのは不自然)。
func (r *AdviceRepository) ListRecentByGoal(ctx context.Context, userID, goalID string, limit int) ([]model.Advice, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+adviceColumns+` FROM advices WHERE user_id = $1 AND goal_id = $2 ORDER BY generated_at DESC LIMIT $3`,
		userID, goalID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAdvices(rows)
}

// GetByID は所有者を問わず1件取得する。所有者チェック(403)はservice層で行うため、
// 「存在しない(404)」と「自分のものではない(403)」を区別できるようにしている
// (runs/goalsと同じ方針、docs/api.md 4.)。
func (r *AdviceRepository) GetByID(ctx context.Context, id string) (*model.Advice, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+adviceColumns+` FROM advices WHERE id = $1`, id)
	return scanAdvice(row)
}

// UpdateFeedback はフィードバックの3カラムのみを更新する(docs/adr/019)。
// AI生成済みの本文(advice_text・next_menu等)には触れない。
func (r *AdviceRepository) UpdateFeedback(ctx context.Context, id string, isHelpful bool, comment *string) (*model.Advice, error) {
	row := r.pool.QueryRow(ctx,
		`UPDATE advices
		 SET is_helpful = $1, feedback_comment = $2, feedback_at = now()
		 WHERE id = $3
		 RETURNING `+adviceColumns,
		isHelpful, comment, id)
	return scanAdvice(row)
}
