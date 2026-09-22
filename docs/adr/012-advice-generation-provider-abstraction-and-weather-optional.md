# ADR 012: フェーズ5(AI提案)の実装方針 — プロバイダ抽象化とregion未設定時の扱い

## ステータス
決定済み

## 背景
フェーズ5(`GET /advices/latest`)の実装にあたり、docs/api.md・docs/database.mdには生成フローと
テーブル構造は明記されていたが、以下2点は実装時に決める必要があった。

1. ADR 002で「開発段階はGemini、本番前にClaude/OpenAIと比較して再決定」と決まっているが、
   将来のプロバイダ切り替えをどう安全に行うか
2. `users.region`が未設定のユーザーに対して、天候情報が無いままAI提案を生成してよいか

## 検討した選択肢

### AI・天候呼び出しの抽象化
- **A: AdviceServiceが直接Gemini/OpenWeatherMapのHTTPクライアントを呼ぶ**: 実装は単純だが、
  本番前にプロバイダを変更する際にAdviceService自体の書き換えが必要になる
- **B: `AdviceGenerator`・`WeatherClient`インターフェースを定義し、AdviceServiceはそれにのみ依存する**:
  docs/architecture.md 7.で「AI呼び出しをservice層に閉じ込める」「repositoryはインターフェースで
  抽象化」と方針があり、同じ考え方をAI・天候クライアントにも適用できる

### region未設定時の扱い
- **A: regionが無い場合はAI提案自体をブロックする(needs_active_goalと同様のガイダンス状態を追加)**:
  安全だが、docs/api.md 3.の生成条件(記録3件以上・目標active)に「regionの設定」を追加するのは
  ドキュメントに無い制約を勝手に追加することになる
- **B: regionが無い場合は天候APIを呼ばず、天候情報なしでAI提案を生成する**: docs/requirements.md
  6.では天候はあくまで「反映する」機能であり必須入力とは書かれていない。降ってこないなら
  「天候情報なし」として素直にプロンプトに含めない方が実装もシンプル

## 決定
- Bを採用(抽象化)。`internal/service/ai_client.go`に`AdviceGenerator`インターフェースと
  `GeminiClient`実装、`internal/service/weather_client.go`に`WeatherClient`インターフェースと
  `OpenWeatherMapClient`実装を置く。`AdviceService`はこの2つのインターフェースのみに依存し、
  `main.go`で具象実装を注入する(DI)
- Bを採用(region未設定時)。`model.WeatherContext`に`HasWeather bool`を持たせ、regionが空文字/nil
  の場合は天候API呼び出し自体をスキップし、`HasWeather: false`のままDBに保存する
  (jsonbカラムとしてはNULLになる。`WeatherContext.Value()`参照)

## 理由
1. インターフェース抽象化は「本番前にAPIプロバイダを変更しても影響範囲が限定的」という
   ADR 002の前提を実際にコードで担保するために必要
2. 専用SDKは導入せず`net/http`で直接REST呼び出しする実装とした。Gemini公式Go SDKも存在するが、
   個人開発MVPで依存を増やすほどの恩恵がなく、レスポンス形式もシンプルなJSONのため
   標準ライブラリで十分(CLAUDE.mdの過剰設計をしない方針)
3. AIへのプロンプトはJSON出力(`responseMimeType: application/json`)を強制することで、
   `advice_text`/`next_menu`をパース失敗なくそのままDBのjsonb列に保存できるようにした
4. region未設定でAI提案自体をブロックすると、「地域設定は任意項目」というdocs/database.md 3.1の
   前提と矛盾する。天候情報が無くても記録と目標だけで一定の提案は可能なため、機能を止めない方を選んだ

## 影響
- `GEMINI_API_KEY`・`OPENWEATHERMAP_API_KEY`を`internal/config`の必須環境変数に追加した。
  未設定だと起動時に落ちる(DATABASE_URL等と同じ扱い)。ローカルの`.env`に追記が必要
- 本番移行時にAPIプロバイダを変更する場合、`AdviceGenerator`実装を追加して`main.go`の
  DI箇所を差し替えるだけで済む想定(AdviceService・handlerの変更は不要)
- 天候取得に失敗した場合(region設定済みだがAPIエラー)は、docs/api.md 4.の方針通り500として
  扱う(regionが「未設定」の場合とは区別する)
