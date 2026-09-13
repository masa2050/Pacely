# ADR 006: マイグレーション管理にgolang-migrateを採用

## ステータス
決定済み

## 背景
`docs/database.md`のテーブル定義を実際のPostgreSQL(ローカルDocker / 本番Supabase)へ適用する仕組みが必要だった。

## 検討した選択肢
- **A: golang-migrate(CLI)**: SQLファイルをup/downのペアで書き、バージョン管理する専用ツール。Goエコシステムで広く使われている
- **B: 自前のSQLファイル + 起動時適用**: 外部ツールを入れず、backend起動時にSQLファイルを順番に実行する自作コード。依存が増えない代わりに、ロールバックやバージョン管理を自分で実装する必要がある
- **C: ORMのマイグレーション機能(例: GORM)**: ORM導入とセットになる。Pacelyでは`database/sql`系(pgx)を素で使う方針のため、ORM導入という大きな変更を伴う

## 決定
Aのgolang-migrate CLIを採用する。

## 理由
1. SQLをそのまま書けるため、`docs/database.md`のテーブル定義と1対1で対応させやすく、後から見返しやすい
2. up/downの両方を書く型が身につき、マイグレーションの基本(schema_migrationsテーブルでのバージョン管理、ロールバック)を学べる
3. CLIツールとして`go install`で導入するだけで、backendのgo.modに依存が増えない(実行時にリンクされるライブラリではないため)

## 影響
- マイグレーション適用は手動コマンド実行(`migrate -path migrations -database "$DATABASE_URL" up`)。CI/CDに組み込む場合は別途スクリプト化が必要(フェーズ6で検討)
- SQLを直接書くため、DB方言(PostgreSQL固有の型・構文)への理解がそのまま問われる
