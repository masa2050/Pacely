-- docs/database.md 3.4 advices に対応。
--
-- user_id/goal_id は runs・goalsと同様に外部キーを張る(docs/adr/008参照)。
-- goal_idはnullable: 目標未設定時点ではadvicesは生成されない(GET /advices/latest の
-- 生成判定でstatus=activeのgoalが無い場合は生成をスキップするため)が、将来「目標なしで
-- 一般的なアドバイスのみ返す」仕様に変えた場合に備えnullを許容しておく。
-- weather_contextもnullable: users.regionが未設定のユーザーは天候APIを呼ばずに
-- 生成する(docs/adr/012参照)ため、天候情報が存在しない場合がある。
CREATE TABLE advices (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id),
    goal_id uuid REFERENCES goals(id),
    advice_text text NOT NULL,
    next_menu jsonb NOT NULL,
    weather_context jsonb,
    generated_at timestamptz NOT NULL DEFAULT now()
);

-- 最新提案の取得・履歴表示のためのインデックス(docs/database.md 4.)。
CREATE INDEX idx_advices_user_id_generated_at ON advices (user_id, generated_at);
