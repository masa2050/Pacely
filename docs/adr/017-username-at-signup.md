# ADR 017: ユーザーネームをサインアップ時に登録する方式

## ステータス
決定済み

## 背景
フェーズ7-3の実装当初は「usernameは設定画面で後から編集する」方式にしたが、ユーザーネームは
本来アカウント作成のタイミングで決めるものであり、サインアップ直後に未設定のまま使い始められる
現状の設計は不自然という指摘があった。既にログイン中のユーザーが存在する状態で、サインアップ画面に
入力欄を追加する方式に変更できるか、また既存ユーザーのログインに影響が出ないかを検討した。

## 検討した選択肢

### usernameの受け渡し方法
- **A: サインアップ成功後、フロントエンドから`PUT /users/me`でusernameを保存する**:
  実装は最小(フロントエンドのみ)。ただし`devlog.md`に残っている本番前TODO「Supabase
  "Confirm email" をONに戻す」を実施すると、`signUp`はメール確認が完了するまでセッション
  (JWT)を返さなくなる。認証必須の`PUT /users/me`をサインアップ直後に呼べなくなり、
  入力したusernameが確認メールのリンクを踏むまでの間、宙に浮いて失われる
- **B: `supabase.auth.signUp`の`options.data`にusernameを渡し、Supabase Auth側の
  `user_metadata`として保存する**: メール確認の有無に関わらずsignUp呼び出し時点で
  Supabase Auth側に保存されるため、確認を挟んで後日ログインしても値が失われない。
  ログイン後に発行されるJWTの`user_metadata`クレームから読み取れるため、バックエンドの
  get-or-create(ADR-007)のタイミングで`public.users`テーブルへ書き込める

### 既存ログイン中ユーザーへの影響
- サインアップフローの変更であり、ログイン(`signInWithPassword`)側のコード・JWT検証
  (`internal/middleware/auth.go`)には手を入れていない。既存ユーザーは`username`列が
  `NULL`のままDBに残り、ログインは今まで通り成功する
- `username`列はNOT NULL制約を付けていない(migrations/000006)。既存ユーザーに対して
  一括で値を埋める移行(バックフィル)は行わず、設定画面から任意のタイミングで設定できる状態を維持する

## 決定
- Bを採用する。`AuthForm.tsx`のサインアップフォームに「ユーザーネーム」の入力欄を必須項目として追加し、
  `supabase.auth.signUp({ email, password, options: { data: { username } } })`で渡す
- バックエンドの`internal/middleware/auth.go`(JWT検証)で`user_metadata.username`クレームを
  読み取り、`ContextUsernameKey`としてリクエストコンテキストに格納する
- `internal/service/user.go`に`GetOrCreateMeWithUsername`を追加し、`GET /users/me`
  (ADR-007のget-or-create)で行を新規作成する際にのみusernameを一緒に設定する。
  既に行が存在する場合(設定画面から変更済みの場合を含む)は、JWTのuser_metadataより
  DB側の値を優先する(そのまま何もしない)
- 設定画面(`Settings.tsx`)からの編集機能はそのまま残す。サインアップ時に決めた
  ユーザーネームも後から変更できるようにするため、また既存ユーザー(サインアップ時に
  入力欄が無かったユーザー)がユーザーネームを設定する唯一の手段でもあるため

## 理由
1. "Confirm email" を本番でONに戻す前提(devlog.mdのTODO)がある以上、サインアップ直後の
   `PUT /users/me`に依存する設計は本番で機能しなくなるリスクが高い。Supabase Auth自体の
   仕組み(user_metadata)に載せておけば、メール確認のタイミングに関係なく値が保持される
2. `run.go`/`goal.go`/`advice.go`が内部的に呼んでいる`GetOrCreateMe`はusernameを必要としないため、
   シグネチャを変えず`GetOrCreateMeWithUsername`という別メソッドを追加するだけに留めた。
   3ファイルの呼び出し元を変更する必要がなく、影響範囲を`GET /users/me`のハンドラのみに限定できる
3. 既存ユーザーの救済(NULL許容・設定画面からの後編集)を維持することで、サインアップ時の
   入力欄追加が既存アカウントのログイン可否に一切影響しないようにした

## 影響
- `AuthForm.tsx`のサインアップフォームに必須のユーザーネーム入力欄が増える(ログイン・
  パスワード再設定フォームには影響しない)
- 新規ユーザーは初回`GET /users/me`(ログイン直後にDashboardが呼ぶ)のタイミングで
  usernameが設定された状態でプロフィール行が作られる
- 既存ユーザーは引き続き設定画面から設定する
