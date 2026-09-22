# ADR 008: runsテーブルはusers(id)へのFKを張り、記録作成時にget-or-createを先に呼ぶ

## ステータス
決定済み

## 背景
フェーズ2でrunsテーブル(docs/database.md 3.2)を実装するにあたり、`user_id`にDBレベルの外部キー制約を張るかどうかを検討した。

ADR 007では「`users.id` → `auth.users.id`」のFKを見送っている。理由はローカル開発用DB(素のPostgres)に`auth`スキーマ自体が存在せず、マイグレーションが失敗するためだった。一方、`users`と`runs`は両方ともPacely自身が管理するテーブルであり、ローカル・本番どちらの環境にも実体として存在する。ADR 007の見送り理由はここには当てはまらない。

ただし`users`テーブルはget-or-create方式(初回`GET/PUT /users/me`アクセス時に行が作られる、ADR 007)のため、ユーザーが一度も`/users/me`を呼ばずに先に`POST /runs`を呼んだ場合、`users`行が存在せずFK制約違反になる懸念があった。

## 検討した選択肢
- **A: FKを張らない**: ADR 007と同じ理由(整合性はJWT検証で担保)で押し通す。実装は簡単だが、`users`と`runs`は同一DB内のテーブル同士であり、FKを張らない技術的理由がない
- **B: FKを張り、`POST /runs`のservice層で記録作成前に`users`行のget-or-createを呼ぶ**: DBレベルの整合性を保ちつつ、ユーザー側のフロー(先に`/users/me`を呼ぶ必要がある、という制約)を増やさない

## 決定
Bを採用する。`runs.user_id`は`users(id)`へのFK制約を張る。`RunService.Create`は記録保存の前に`UserService.GetOrCreateMe`を呼び、`users`行の存在を保証してから`INSERT`する。

## 理由
1. 同一DB内のテーブル同士でFKを張らない理由がない。DB制約で守れる整合性はDBに守らせる方が、アプリケーションコードだけに頼るより堅牢
2. get-or-createは既にADR 007で採用済みのパターンであり、`RunService`から`UserService`を呼ぶだけで実現できる。新しい仕組みを追加する必要がない
3. フロントエンド側は「記録前に必ずプロフィール画面を開く」といった追加のフローを強制されずに済む

## 影響
- `RunService`は`UserService`に依存する(コンストラクタで注入)。今後、記録に類する他のテーブル(goalsなど)を追加する際も同じパターンを踏襲する
