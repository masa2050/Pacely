# ADR 023: ユーザー向けエラー文言の日本語化と重複メール検出

## ステータス
決定済み

## 背景
フェーズ9-1として、画面に出るエラー文を日本語に統一する。調査の結果、英語や開発者向け表記が
ユーザーに見える経路が4つあった。

1. Supabase Authの`error.message`(例: "Invalid login credentials"、"User already registered")を
   そのまま表示していた(AuthForm、ResetPasswordForm)
2. ネットワーク断時のfetchの`TypeError`("Failed to fetch")が各コンポーネントの`err.message`で
   そのまま表示されていた
3. BEの`ErrInvalidInput`は`err.Error()`をそのまま返しており、"invalid input: distance_km は…"と
   英語のsentinel文言・フィールド名・"(docs/adr/003)"がユーザーに見えていた
4. `adviceToError`のdefault分岐が`"…: "+err.Error()`を連結しており、DBエラーの文面が漏れうる
   (8-1で塞いだ外部APIエラーと同種の穴)

加えて、サインアップ時の重複メール検出(9-2メモ)も扱った。`signUp()`の`error`有無だけを見ており、
登録済みメールでも「登録しました」と出る可能性があった。

## 検討した選択肢

### FEのエラー文変換
- **A: 各コンポーネントで個別に日本語メッセージを書く**: 約20箇所の`err.message`直表示があり、
  取りこぼしと文言のばらつきが出る
- **B: `lib/errors.ts`に`toUserMessage(err)`を1つ作り、全箇所がそれを通す**: 文言が1ファイルに集まり、
  想定外のエラーも汎用の日本語に落ちる(英語の生メッセージが画面に出ない)

### BEの入力エラー文
- **A: service層のメッセージはそのまま(開発者向け)にし、FEで変換する**: FEがフィールド名ごとの
  対応表を持つことになり、BEの変更のたびにFEも直す必要がある
- **B: service層のメッセージ自体をユーザー向け日本語にし、handlerでsentinel文言のプレフィックスだけ除く**

### 重複メールの判定
- **A: `identities`が空配列かどうかだけで判定する**
- **B: `error.code`(`user_already_exists`)と`identities`空配列の両方で判定する**

## 決定
- FEは`toUserMessage`に集約(B)。`ApiError`のmessageはBEが日本語で返す前提でそのまま通し、
  Supabase Authのエラーは`error.code`で日本語に対応づける。未知のcode・TypeError・その他は定型文に落とす
- BEの`ErrInvalidInput`メッセージをユーザー向け日本語にし、handlerの`invalidInputMessage`で
  "invalid input: "を除いて返す(B)。`adviceToError`のdefaultは定型文にして詳細はログへ
- 重複メールは両方で判定する(B)
- `authorizedFetch`の「ログインしていません」は`ApiError(401)`にして、`toUserMessage`が
  「`Error`は全部汎用文にする」ルールのまま日本語を通せるようにした
- サインアップ成功時、セッションが無ければ(Confirm email有効)「確認メールを送信しました」と出し分ける

## 理由
1. Supabase Authの挙動はConfirm emailの設定で変わる。実機確認では、Confirm email無効の開発用プロジェクトは
   登録済みメールに422 `user_already_exists`を返し、`identities`は使われなかった。一方、ユーザー列挙攻撃対策の
   ダミー成功(`identities: []`)はConfirm email有効の環境で起きる仕様のため、どちらの環境でも
   正しく動くよう両方で判定する必要がある
2. 想定外のエラーを「定型の日本語に落とす」方針にすれば、未知のcodeが来ても英語の生メッセージや
   内部情報が画面に出ない
3. 入力エラーの理由はBEの検証ロジックが一番よく知っているため、文言もBE側に置く方が
   FEに対応表を持たせるより保守しやすい

## /code-review 後の追加対応
- Echoが自動生成する英語のエラー文("Not Found"・Recoverが捕まえたpanicの"Internal Server Error"等)が
  素通りしていたため、`handler.HTTPErrorHandler`を`e.HTTPErrorHandler`に設定した。messageが
  `http.StatusText`と一致する場合のみ日本語の定型文に置き換え、各ハンドラの日本語messageは通す。
  echo.HTTPError以外の素のエラーはログに残して500の定型文にする
- `invalidInputMessage`は先頭一致ではなく「sentinel文言より後ろ」を取り出す方式にし、途中で
  別のエラーにラップされても英語が漏れないようにした
- goal/run/userハンドラのdefault分岐に`log.Printf`を追加した(従来は原因が記録されなかった)
- 保存済みの目標に未対応の`goal_type`があるケースは、ユーザー入力の誤りではなくデータ不整合のため、
  400(`ErrInvalidInput`)をやめて通常のエラー(500+ログ)にした
- FEの`toUserMessage`にSupabase Authの5xxとcode(`validation_failed`/`otp_expired`)を追加し、
  `ApiError`のmessageは「空でない文字列」のときだけ採用する

### 2回目の/code-review後
- `HTTPErrorHandler`は`echo.ErrNotFound`等のグローバル変数(全リクエストで共有)を直接書き換えると
  データ競合になるため、コピーに対して文言を差し替える。`Committed`済みなら何もせず、
  `Internal`が`*HTTPError`の場合は標準ハンドラと同様に内側を対象にする
- `GoalInput.validate()`が空文字しか弾かず未知の`goal_type`を保存できてしまうのが根本原因だったため、
  `goalTypeDistanceKm`にある種類のみ許可するよう修正した(`GetProgress`の500+ログは防御として残す)
- 通信エラーの判定を「TypeError一般」から、`authorizedFetch`が`fetch`失敗を変換する`NetworkError`に変更した
  (TypeErrorは通信以外のバグでも投げられ、原因を隠すため)
- 各`*ToError`のdefault分岐のログ出力を`internalError`ヘルパーに集約し、書き忘れを防ぐ
- 見送り: `ErrInvalidInput`を型付きエラーにする案。sentinel文言の後ろにそのまま返す前提は
  `invalidInputMessage`のコメントに明記し、現時点では全箇所が日本語のみで組み立てているため

## 受容したリスク
- 重複メールを「既に登録済み」と表示するため、登録フォームからメールアドレスの登録有無が推測できる
  (Supabaseが`identities: []`のダミー成功を返すのはまさにこれを防ぐため)。9-2の仕様として、
  ユーザーが入力ミスに気づける利便性を優先すると決めた。問題になる規模になったら、
  「登録を受け付けました。メールを確認してください」の中立文言に変更する

## 影響
- 新規: `frontend/src/lib/errors.ts`、`backend/internal/handler/errmsg.go`
- docs/api.md 4.のレスポンス形式を実装(`message`キー)に合わせて修正し、messageの方針を追記
- **未検証**: `identities: []`の経路(Confirm email有効の環境)は開発用プロジェクトで再現できなかった。
  本番(確認メール有効の想定)で重複メールのサインアップを試して確認する必要がある
- 既存の重複アカウントの統合・削除は引き続きスコープ外
