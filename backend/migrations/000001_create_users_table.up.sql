-- docs/database.md 3.1 users に対応。
--
-- id は Supabase Auth が発行する user id (JWTのsubクレーム) をそのまま使う。
-- ローカル開発用DBには Supabase の auth.users テーブルが存在しない(素のPostgres)ため、
-- auth.users への外部キー制約はあえて付けない。整合性はアプリ層(JWT検証)で担保する。
CREATE TABLE users (
    id uuid PRIMARY KEY,
    region varchar(100),
    created_at timestamptz NOT NULL DEFAULT now()
);
