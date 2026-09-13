# 開発ログ(devlog)

実装中に「詰まった点・調べたこと・設計を変えた判断」を時系列で残す。
面接などで「なぜその技術/設計にしたか」「どう問題解決したか」を具体的に話せるようにするのが目的。

書き方: 新しい項目を上に追加する。1項目 = 「状況 / 原因 / 対応 / 学び」。

---

## 2026-09-14 フェーズ1: マイグレーション基盤 + usersテーブル + GET/PUT /users/me

- **マイグレーションツール**: `golang-migrate` のCLIを採用(`go install .../cmd/migrate@latest`)。
  `backend/migrations/` に `NNNNNN_名前.up.sql` / `.down.sql` を置き、
  `migrate -path migrations -database "$DATABASE_URL" up` で適用する。
- **usersテーブルの id に auth.users への外部キーを付けなかった**:
  ローカル開発用DBは素のPostgres(Supabaseのauthスキーマが存在しない)なので、
  FK制約を付けるとローカルで動かなくなる。整合性はJWT検証(アプリ層)で担保する方針にした。
  本番のSupabaseでも同じスキーマを使い回せるようにするための判断。
- **get-or-create方式にした理由**: Supabase Auth側でユーザーが作られても、
  Pacely独自の `users` テーブルの行は自動生成されない(サインアップ時のWebhook等は
  MVPでは作らない)。そこで「初回の `GET /users/me` アクセス時に行が無ければ作る」
  実装にし、フロント側にプロフィール作成専用の処理を増やさないようにした。
- **確認**: 同じユーザーで `GET /users/me` を2回叩いても行が重複しない(`created_at` が
  変わらないことで確認)、`PUT /users/me` で `region` 更新、`region` 空文字は400、を確認。

## 2026-09-11 フェーズ1: JWT検証を HS256 から JWKS(ES256) 方式へ変更

- **状況**: Supabase の「JWT Secret」を使う共有鍵(HS256)方式で検証ミドルウェアを実装したが、
  実際に発行されたトークンを検証すると必ず 401 になった。
- **原因**: トークンの header を確認したら `alg: ES256`(非対称鍵署名)だった。
  近年作成された Supabase プロジェクトは、共有シークレットではなく署名鍵ペアで JWT を署名し、
  公開鍵を JWKS エンドポイント(`/auth/v1/.well-known/jwks.json`)で配布する方式がデフォルトになっている。
- **対応**: `MicahParks/keyfunc` で JWKS を取得して公開鍵で署名検証する方式に作り直した。
  鍵ローテーションに追従するため定期リフレッシュ付き。
  ミドルウェアでは `alg` を ES256 に固定(アルゴリズム混同攻撃対策)、`aud=authenticated`・`exp` も検証。
  設定は `SUPABASE_JWT_SECRET` を廃止し `SUPABASE_URL` から JWKS URL を導出する形に変更。
- **学び**: JWT 検証は「ライブラリに丸投げ」ではなく、まず対象トークンの header(`alg`,`kid`)を
  自分でデコードして確認するのが早い。対称鍵方式と非対称鍵方式で実装がまったく変わる。

## 2026-09-11 フェーズ1: Supabase の "Confirm email" 有効でテストログインが弾かれた

- **状況**: 検証用トークンを取ろうとしたら、`/auth/v1/token` が `email_not_confirmed` を返した。
- **原因**: Supabase の Email プロバイダ設定で "Confirm email" が有効。確認メールのリンクを踏むまで
  ログイン(セッション発行)ができない。
- **対応**: 開発中は Authentication > Sign In / Providers > Email の "Confirm email" を OFF に。
  本番前に ON へ戻す(devlog に TODO として残す)。
- **学び**: 認証まわりはコードだけでなく、IdP(Supabase)側の設定でも挙動が大きく変わる。
  「ローカルで疎通確認を最優先」という実装計画の方針は正しかった。

## 2026-09-11 フェーズ0: `.env.example` の置き場所をパッケージ単位に変更

- **状況**: 最初はリポジトリ直下に `.env.example` を1つ置いた。
- **原因**: バックエンド(Go)は `DATABASE_URL` や `SUPABASE_URL`、フロント(Vite)は `VITE_` 接頭辞の
  変数と、必要な環境変数が別物。1ファイルにまとめると「どっちで使う値か」が曖昧になる。
- **対応**: `backend/.env.example` と `frontend/.env.example` に分割。
- **学び**: モノレポでは環境変数もパッケージ境界で分けたほうが管理しやすい。

## 2026-09-11 フェーズ0: 環境構築での詰まり(Go 未インストール / Docker 停止)

- Go が未インストールで、`winget` も対話プロンプトで失敗したため手動インストール(go1.27.1)。
- Docker Desktop が停止していたため、起動を待つ処理を挟んでから `docker compose up -d`。
- **学び**: 環境構築フェーズは「動く前提」を1つずつ確認する。ツールのバージョン確認を最初にやる。

---

## TODO(本番前に対応)

- [ ] Supabase "Confirm email" を ON に戻す
- [ ] AI API の spending limit 設定(フェーズ6)
