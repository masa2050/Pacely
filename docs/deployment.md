# Pacely 本番デプロイ手順

フェーズ6の完了条件(本番URLで実際に使える状態)を満たすための手順。
docs/architecture.md 6.の通り、Vercel(FE)・Railway(BE)・Supabase(DB/Auth)の3サービスを使う。

## 0. 前提

- Supabaseプロジェクトはローカル開発時点ですでに作成済み(`frontend/.env`の`VITE_SUPABASE_URL`が指す
  プロジェクトをそのまま本番でも使う。個人開発MVPのため環境を分けない方針、docs/architecture.md 6.)
- ローカルのアプリデータ(runs/goals/advices/users)はDocker上のPostgreSQLに保存しているだけで、
  Supabase側のPostgreSQLにはまだテーブルが無い。本番移行時に初めてSupabase側へマイグレーションを適用する

## 1. Supabase側の準備

1. **DB接続文字列を取得する**: Supabaseダッシュボード > Project Settings > Database > Connection string。
   - **「Transaction」モード(pgbouncer経由)は避けること。** pgxはデフォルトでプリペアドステートメントを
     使うが、Transactionモードのpgbouncerはこれに対応しておらず実行時エラーになる。「Direct connection」
     または「Session」モードなど、pgbouncerを経由しない(またはSessionモードで動作する)接続文字列を選ぶ。
     RailwayのBEは常駐サーバーで同時接続数もMVP規模では少ないため、コネクションプーラーを使う必然性がない。
     どれを選ぶべきか画面上の表記で迷った場合は、Supabase側の「プリペアドステートメント対応」に関する
     説明を確認してから決める(UIの選択肢名は変わる可能性がある)
2. **マイグレーションを適用する**(ローカルの`migrate`コマンドから、接続先だけ本番に向けて実行):
   ```
   migrate -path backend/migrations -database "<1で取得したSession接続文字列>" up
   ```
3. **service_roleキーを取得する**: Project Settings > API > `service_role` `secret`(退会機能用、ADR014)
4. **Redirect URLsに本番のVercel URLを追加する**: Authentication > URL Configuration > Redirect URLs に
   `https://<本番のVercelドメイン>` を追加(ADR010。パスワード再設定メールのリンク遷移先として必要)

## 2. Railway(バックエンド)へのデプロイ

1. GitHubリポジトリと連携し、新規サービスを作成。Root Directoryを`backend`に設定する
2. `backend/Dockerfile`があるため、Railwayは自動的にDockerビルドを使う(Nixpacksの自動判定には頼らない)
3. 環境変数を設定する(`backend/.env.example`の一覧に対応):

   | 変数名 | 値 |
   | --- | --- |
   | `DATABASE_URL` | 1-1で取得したSupabaseのSession接続文字列 |
   | `SUPABASE_URL` | `https://xxxxxxxxxxxx.supabase.co`(既存のプロジェクトURL) |
   | `SUPABASE_SERVICE_ROLE_KEY` | 1-3で取得したservice_roleキー |
   | `GEMINI_API_KEY` | 既存のGemini APIキー(ADR013、本番でも継続利用) |
   | `OPENWEATHERMAP_API_KEY` | 既存のOpenWeatherMap APIキー |
   | `FRONTEND_ORIGIN` | 手順3でVercelのURLが確定してから設定(それまでは未設定のままでOK、CORSでフロントからのアクセスが弾かれるだけでBE自体は起動できる) |
   | `PORT` | 設定不要。RailwayがPORTを自動注入し、`main.go`はそれを読む |

4. デプロイ後、RailwayがBEに割り当てたURL(例: `https://xxxx.up.railway.app`)を控えておく
5. `https://<RailwayのURL>/health` にアクセスし、`{"status":"ok","db":"up"}`が返ることを確認する
   (DB接続文字列やSupabase接続の設定ミスはここで気づける)

## 3. Vercel(フロントエンド)へのデプロイ

1. GitHubリポジトリと連携し、新規プロジェクトを作成。Root Directoryを`frontend`に設定する
   (Framework PresetはViteが自動検出される想定)
2. 環境変数を設定する(`frontend/.env.example`の一覧に対応):

   | 変数名 | 値 |
   | --- | --- |
   | `VITE_SUPABASE_URL` | 既存のSupabaseプロジェクトURL(ローカルと同じ値) |
   | `VITE_SUPABASE_ANON_KEY` | 既存のSupabase anonキー(ローカルと同じ値) |
   | `VITE_API_BASE_URL` | 手順2-4で控えたRailwayのURL |

3. デプロイ後、Vercelが割り当てたURL(例: `https://pacely.vercel.app`)を控える

## 4. 本番URL確定後の後始末

デプロイ順序上、Vercel/RailwayのURLは互いに相手のURLが確定してからでないと設定できない箇所がある。
両方デプロイし終えたら、以下を必ず反映する。

1. Railwayの環境変数`FRONTEND_ORIGIN`に、手順3で確定したVercelのURLを設定する(再デプロイが必要)
2. Supabaseダッシュボードの Redirect URLs に、同じくVercelのURLを追加する(未登録だとパスワード再設定メールの
   リンクがSupabase側で拒否される)

## 5. 動作確認チェックリスト

- [ ] 本番URLでサインアップ・ログインができる
- [ ] ログイン後、`GET /users/me`が返り、region設定・保存ができる
- [ ] 記録・目標の作成/一覧表示ができる
- [ ] 記録3件・目標ありの状態で`GET /advices/latest`を呼ぶとAI提案が生成される
- [ ] パスワード再設定メールのリンクから本番URLに戻り、`ResetPasswordForm`が表示される
- [ ] 退会ボタンでアカウントが削除され、同じメールアドレスで再ログインできなくなる(ADR014)
