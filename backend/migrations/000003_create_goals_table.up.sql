-- docs/database.md 3.3 goals に対応。
--
-- user_id は runs と同様に users(id) への外部キーを張る(docs/adr/008参照。
-- goalsもPacely自身が管理するテーブルであり、FKを見送る理由がない)。
-- 記録作成時と同様、goal作成前にservice層でusersのget-or-createを呼ぶことで
-- FK制約違反を防ぐ。
--
-- 「有効な目標は1ユーザー1件のみ」(docs/database.md 3.3)という制約は
-- DBのUNIQUE制約ではなく、service層のトランザクション内(新規作成時に
-- 既存activeをabandonedへ更新してからINSERT)で担保する。部分UNIQUEインデックス
-- (status='active'のみに一意制約)でも実現できるが、個人開発MVPでは
-- アプリケーションロジックで十分と判断した(docs/adr/009参照)。
CREATE TABLE goals (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id),
    goal_type varchar(50) NOT NULL,
    target_time_sec integer NOT NULL,
    target_date date NOT NULL,
    status varchar(20) NOT NULL DEFAULT 'active',
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT goals_status_check CHECK (status IN ('active', 'achieved', 'abandoned')),
    CONSTRAINT goals_target_time_positive CHECK (target_time_sec > 0)
);

-- 有効な目標をユーザーごとに素早く取得するためのインデックス(docs/database.md 4.)。
CREATE INDEX idx_goals_user_id_status ON goals (user_id, status);
