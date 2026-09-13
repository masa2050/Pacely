# ADR 007: usersテーブルはSupabase Authに外部キーを張らず、get-or-createで同期する

## ステータス
決定済み

## 背景
Pacely独自の`users`テーブル(region等の拡張プロフィール)は、認証情報を持つSupabase Authの`auth.users`と紐づく必要がある。この同期方法と、DB制約の張り方を検討した。

## 検討した選択肢
- **A: `users.id`に`auth.users.id`への外部キー制約を張る**: DBレベルで整合性を保証できる。ただしローカル開発用DB(素のPostgresコンテナ)には`auth`スキーマ自体が存在しないため、ローカルでマイグレーションが失敗する
- **B: 外部キーなし。整合性はアプリ層(JWT検証)で担保する**: `users.id`にはSupabase AuthのJWTの`sub`(user id)をそのまま入れるが、DB制約としてのFKは張らない。ローカル(素のPostgres)・本番(Supabase)どちらでも同じマイグレーションが通る
- **サインアップ時の行作成方法として、Webhook: Supabase AuthのWebhook(Auth Hooks)でサインアップ時に自動でusers行を作る**: 確実だが、Supabase側の追加設定(Webhook用エンドポイント、シークレット検証)が必要でMVPには早い
- **get-or-create: `GET /users/me`の初回アクセス時に行が無ければ作る**: 追加のインフラ設定が不要。フロント側の実装も増えない

## 決定
外部キーは張らない(B)。行作成はget-or-create方式にする。

## 理由
1. ローカル開発環境(Docker上の素のPostgres)と本番(Supabase)で同じマイグレーションSQLを使い回せることを優先した
2. Webhookは設定コストに対してMVPでのメリットが小さい。get-or-createなら追加のインフラなしで「初回ログイン時にプロフィール行がある」状態を保証できる
3. 整合性(存在しないuser_idでの操作を防ぐ)はJWT検証によってuser_idの出所が保証されるため、DB制約が無くても実用上問題ない

## 影響
- `users`テーブルの整合性はDBではなくアプリケーションコードで守られている、という前提を今後の実装者(未来の自分)が意識する必要がある
- 将来Webhookに切り替える場合、get-or-createのロジックは「作成はしないが取得は失敗しない」形に変更する
