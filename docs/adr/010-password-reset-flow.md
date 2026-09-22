# ADR 010: パスワードリセットはSupabase Auth標準フローを使い、PASSWORD_RECOVERYイベントで画面を切り替える

## ステータス
決定済み

## 背景
フェーズ1(認証機能)実装時、サインアップ・ログインは実装したが「パスワードを忘れた場合」の導線が漏れていた。Pacelyはルーティングライブラリ(react-router等)を導入しておらず、`App.tsx`が`session`の有無だけで`AuthForm`/`Dashboard`を出し分けるシングルビュー構成になっている。この構成のまま、パスワード再設定用のメールリンクから戻ってきた画面をどう表示するかを検討した。

## 検討した選択肢
- **A: react-routerを導入し、`/reset-password`という専用URLを作る**: URL設計としては自然だが、ルーティングライブラリを1機能のためだけに新規導入することになり、CLAUDE.mdの「過剰設計をしない」方針に反する
- **B: ルーティングは増やさず、Supabaseの`onAuthStateChange`が発火する`PASSWORD_RECOVERY`イベントを検知して、`App.tsx`内で表示コンポーネントを切り替える**: 新しい依存を増やさずに実現できる。Supabase Auth SDKは、パスワード再設定メールのリンク(`redirectTo`で指定したURL)にユーザーが遷移した際、自動的にトークンを検出してセッションを確立し、`PASSWORD_RECOVERY`イベントを発火する仕様になっている

## 決定
Bを採用する。`useSession`フックに`isPasswordRecovery`フラグを追加し、`PASSWORD_RECOVERY`イベントを受け取ったら`true`にする。`App.tsx`はこのフラグが立っている間、`session`の有無に関わらず`ResetPasswordForm`(新パスワード入力→`supabase.auth.updateUser({ password })`)を表示する。

`AuthForm`には`login`/`signup`に加えて`reset-request`モードを追加し、`supabase.auth.resetPasswordForEmail(email, { redirectTo: window.location.origin })`でメール送信を行う。`redirectTo`に`window.location.origin`を使うことで、ローカル(`localhost:5173`)・本番(Vercelドメイン)のどちらでもコード変更なしに動く。

## 理由
1. ルーティングライブラリを増やさずに実現でき、既存のシングルビュー構成(`session`の有無で出し分け)と同じ設計思想を保てる
2. `PASSWORD_RECOVERY`イベントの検知はSupabase Auth SDKの標準的な使い方であり、独自のトークン解析・URLパース処理を書く必要がない(SDKの`detectSessionInUrl`がデフォルトで有効なため)
3. バックエンドの変更は不要(docs/api.md 2.1の方針通り、認証関連はフロントからSupabase Auth SDKを直接呼ぶ構成を維持できる)

## 影響
- Supabaseダッシュボードの Authentication > URL Configuration > Redirect URLs に、ローカル(`http://localhost:5173`)と本番デプロイ後のURLの両方を許可リスト登録する必要がある(登録していないURLへの`redirectTo`はSupabase側で拒否される)。本番URL確定後(フェーズ6)に追加登録が必要
- 将来ページ数が増えてreact-router導入を検討する際は、この`isPasswordRecovery`による分岐もルーティングに置き換えることになる
