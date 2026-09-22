# 開発ログ(devlog)

実装中に「詰まった点・調べたこと・設計を変えた判断」を時系列で残す。
面接などで「なぜその技術/設計にしたか」「どう問題解決したか」を具体的に話せるようにするのが目的。

書き方: 新しい項目を上に追加する。1項目 = 「状況 / 原因 / 対応 / 学び」。

---

## 2026-09-22 フェーズ6: 本番デプロイ(Vercel/Railway)でのトラブル3連発

- **状況**: フェーズ6でVercel(FE)・Railway(BE)へ初回デプロイし、本番URLでログイン後の
  動作確認をしていた。「ログイン後に `Unexpected token '<', "<!doctype "... is not valid JSON`」
  というエラーが発生。

- **問題1: `VITE_API_BASE_URL` のスキーム抜け**
  - **原因**: Vercelの環境変数に `pacely-production.up.railway.app`(先頭の`https://`なし)を
    設定していた。`api.ts`は `${API_BASE_URL}${path}` という単純な文字列結合でURLを組み立てて
    いるため、スキームが無いとブラウザはこれを「相対パス」と解釈し、
    `https://<フロントのドメイン>/pacely-production.up.railway.app/users/me` という壊れた
    URLへリクエストしてしまう。
  - **さらに**: 壊れたそのパスは、直前に追加していた`vercel.json`の
    `"rewrites": [{ "source": "/(.*)", "destination": "/index.html" }]`(SPA用キャッチオール)
    に引っかかり、`index.html`(HTML)がそのまま返ってくる。それを`res.json()`でパースしようと
    して`Unexpected token '<'`エラーになっていた。
  - **対応**: `VITE_API_BASE_URL` を `https://pacely-production.up.railway.app`(スキーム付き)
    に修正。
  - **学び**: フロントのURL組み立てを単純な文字列結合にしていると、環境変数の値のちょっとした
    表記ミス(スキーム抜け)がエラーメッセージ上は全く別の場所(JSON parse error)に見える形で
    表面化する。エラーメッセージだけでなく、実際のRequest URLをNetworkタブで確認するのが
    結局一番早い。

- **問題2: Vercelの環境変数を`Secret`で保存すると`Config`に変更できない**
  - **原因**: 環境変数追加時、Typeを`Secret`のまま保存してしまった。Vercelの仕様上
    `Secret`は書き込み専用(write-only)になり、後から`Config`(通常の公開可能な値)に
    変更できない。`VITE_`接頭辞の値はブラウザに露出する前提のものなので、本来`Config`で
    持つべきだった。
  - **対応**: 一度削除して、Typeを`Config`にして作り直した。
  - **学び**: `VITE_`(や他フレームワークの`NEXT_PUBLIC_`等)接頭辞の値は「公開されることが
    前提の値」なので、Secret種別で保存するとかえって身動きが取れなくなる。保存前にTypeを
    確認する癖をつける。

- **問題3: RailwayのCORS許可オリジンに末尾スラッシュが付いていた**
  - **状況**: 問題1・2を直しても`Failed to fetch`(CORSエラー)が発生。
  - **原因**: Railwayの`FRONTEND_ORIGIN`環境変数に`https://pacely-lake.vercel.app/`
    (末尾スラッシュ付き)を設定していた。ブラウザが送る`Origin`ヘッダーは常に
    スキーム+ホストのみで末尾スラッシュを含まないため、バックエンド側の完全一致比較
    (`middleware.CORSWithConfig`の`AllowOrigins`)で不一致になり弾かれていた。
  - **副次的な混乱**: Vercelの「Deployments」一覧から「Visit」で開くと、固定の本番ドメイン
    (`pacely-lake.vercel.app`)ではなく、デプロイ固有のランダムなURL
    (`pacely-xxxxxxxx-<team>.vercel.app`)が開くため、一瞬「まだ反映されていない別のビルドを
    見ているのでは」と誤解しかけた。実際は中身は同じ最新ビルドで、URLが違うだけだった。
  - **対応**: `FRONTEND_ORIGIN`から末尾スラッシュを削除。固定の本番ドメイン
    (`pacely-lake.vercel.app`)でアクセスして確認した。
  - **学び**: CORSの`AllowOrigins`は完全一致(パスやスラッシュも含めて)である点を忘れがち。
    環境変数にURLを設定するときは、末尾スラッシュの有無をコピペ元と揃える意識が必要。

- **総括の学び**: 本番デプロイで起きたトラブルは3つとも、コード自体のバグではなく
  「環境変数の表記ゆれ」(スキーム抜け・Type選択ミス・末尾スラッシュ)が原因だった。
  ローカル開発では`.env`を直接書くので気づきにくいが、本番デプロイでは環境変数の値を
  UI経由で手入力するため、こうした表記ミスが起きやすい。次回以降は設定直後に
  ブラウザの開発者ツール(Networkタブ)でRequest URLとCORSエラーの有無を必ず確認する。

## 2026-09-14 フェーズ1: FE認証画面 + CORS設定漏れ

- **状況**: ログイン/サインアップ画面を実装し、ブラウザで動作確認しようとした。
- **見つけた問題**: バックエンド(Echo)にCORSミドルウェアを設定していなかった。
  ブラウザは `Authorization` ヘッダー付きリクエストを送る前に preflight(OPTIONS)を
  送るため、CORS設定が無いと `/users/me` などすべてのAPIが失敗する。
  curlでの検証だけでは気づけなかった(curlはpreflightを送らないため)。
- **対応**: `middleware.CORSWithConfig` を追加。許可オリジンは `FRONTEND_ORIGIN` 環境変数
  (未設定時は `http://localhost:5173`)から読む。
- **確認方法**: Playwright(chromium)でヘッドレスブラウザを操作し、実際に
  サインアップ→ログアウト→再ログイン→region更新まで一通り実行。ネットワークログで
  CORSエラーが出ていないこと、`/users/me` が200を返すことを確認した。
- **学び**: バックエンド単体のAPIテスト(curl)と、ブラウザ経由の結合テストでは
  検出できる問題の種類が違う。特にCORSはブラウザからでないと再現しない。
- **副産物**: Supabaseの`signUp`は「確認メール不要」設定だと、サインアップ即セッションが
  張られる(ログイン操作なしでダッシュボードに遷移する)。想定通りの挙動。

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
