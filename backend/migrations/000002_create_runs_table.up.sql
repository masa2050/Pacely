-- docs/database.md 3.2 runs に対応。
--
-- gen_random_uuid() は pgcrypto 拡張が提供する関数。ローカル(素のPostgres)・
-- 本番(Supabase)どちらにも標準で存在するため、追加の依存なしで使える。
--
-- user_id は users(id) への外部キーを張る(users・runsは両方ともPacely自身の
-- テーブルであり、ADR 007でFKを見送った auth.users の問題 -- ローカルDBに
-- authスキーマが無い -- は発生しない)。ただしusersはget-or-create方式
-- (初回 GET/PUT /users/me で作成)のため、記録作成時にusers行がまだ無い
-- ケースがありうる。これはservice層で「記録作成前にget-or-createを呼ぶ」
-- ことで担保する(docs/adr/007-users-profile-sync-strategy.md参照)。
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id),
    distance_km numeric(6, 2) NOT NULL,
    duration_sec integer NOT NULL,
    -- distance/durationから計算可能だが、一覧表示のクエリコストを下げるため
    -- 保存時に計算して持つ(docs/database.md 5. 設計上の注意点)。
    pace_sec_per_km numeric(8, 2) NOT NULL,
    -- 体感的きつさ(Borg CR10, 1-10, 任意)。docs/adr/003-rpe-cr10-scale.md参照。
    rpe integer,
    run_date date NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT runs_rpe_range CHECK (rpe IS NULL OR (rpe BETWEEN 1 AND 10)),
    CONSTRAINT runs_distance_positive CHECK (distance_km > 0),
    CONSTRAINT runs_duration_positive CHECK (duration_sec > 0)
);

-- 記録一覧をユーザー・日付で絞り込むためのインデックス(docs/database.md 4.)。
CREATE INDEX idx_runs_user_id_run_date ON runs (user_id, run_date);
