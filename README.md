# Pacely

目標レースから逆算して「次に何を走るべきか」をAIが提案するランニングコーチアプリ。

**本番環境**: https://pacely-lake.vercel.app

## これは何か

Pacelyはランニング**記録**アプリではなく、ランニング**AIコーチ**アプリである。

市民ランナーの多くは練習メニューを自己流で組んでおり、「このままで目標に届くのか」が分からない。
一方で専属コーチをつける余裕はない。Pacelyは、記録した練習内容と設定した目標（種目・目標タイム・
目標日）をAIに渡し、**次回の具体的な練習メニュー（種別・距離・ペース）とアドバイス**を生成する。

記録の蓄積そのものではなく「目標逆算型の提案」が価値の中心であり、この前提を外れる機能は
MVPに含めていない（→ [docs/requirements.md](docs/requirements.md)）。

## 画面

PC幅では記録・目標/進捗・AI提案の3カラム、スマホ幅では縦1列に切り替わるレスポンシブ構成。

<p align="center">
  <img src="docs/images/dashboard.png" alt="ダッシュボード(PC幅): プロフィール・ランニング記録・目標設定・進捗確認・AI提案を3カラムで表示" width="720">
</p>

<table>
  <tr>
    <td align="center" valign="top" width="34%">
      <img src="docs/images/advice.png" alt="AI提案カード: アドバイス・本日の練習メニュー・フィードバック" width="100%"><br>
      <sub><b>AI提案</b><br>アドバイスと本日の練習メニュー。<br>役に立ったかをフィードバックできる</sub>
    </td>
    <td align="center" valign="top" width="38%">
      <img src="docs/images/history.png" alt="AI提案の履歴: 日付・メニュー・評価の1行一覧" width="100%"><br>
      <sub><b>提案履歴</b><br>過去の提案を1行で一覧し、<br>クリックで全文を展開</sub>
    </td>
    <td align="center" valign="top" width="28%">
      <img src="docs/images/mobile.png" alt="スマホ幅のダッシュボード" width="100%"><br>
      <sub><b>スマホ幅</b><br>縦1列に切り替わる</sub>
    </td>
  </tr>
</table>

## 主な機能

| 機能 | 内容 |
| --- | --- |
| 認証 | Supabase Authによるメール/パスワード登録・ログイン、パスワード再設定・変更、退会 |
| ランニング記録 | 距離・時間・日付・RPE（体感的きつさ、CR10スケール）の手動入力。ペースは保存時に算出 |
| 目標設定 | 種目（フル/ハーフ/10km/5km）・目標タイム・目標日。有効な目標は常に1件 |
| 進捗確認 | 直近の記録から平均ペースを出し、目標ペースとの差分・残日数を表示 |
| **AI提案（中核）** | 記録・目標・天候をもとにアドバイスと次回練習メニューを生成。ジョグ/ペース走/インターバル/ロング走/ビルドアップ走/休養を提案し、ビルドアップ走は区間ごとの内訳付き |
| AI提案の履歴 | 過去の提案を日付・メニュー・評価の1行一覧で振り返り、クリックで全文を展開（専用画面） |
| AI提案へのフィードバック | 「役に立った/役に立たなかった」＋任意コメントを送信（提案精度改善の基盤） |
| 天候考慮 | ユーザーが選択した都道府県の天候をAIプロンプトに反映 |

## 技術スタック

| 領域 | 技術 | 選定理由 |
| --- | --- | --- |
| フロントエンド | React + TypeScript (Vite) | Vercelとの相性、学習価値 |
| バックエンド | Go + Echo | インターンでの使用経験あり。シンプルな文法で個人開発と相性が良い（[ADR 001](docs/adr/001-go-instead-of-kotlin-springboot.md)） |
| DB | PostgreSQL (Supabase) | リレーショナルなデータ構造に適し、無料枠が個人開発に十分 |
| 認証 | Supabase Auth (JWT) | 自前実装よりセキュリティリスクを抑える。検証はJWKS/ES256（[ADR 005](docs/adr/005-jwt-verification-jwks-es256.md)） |
| AI | Google Gemini API | 継続的な無料枠があり、従量課金の事故が起きない。混雑時は予備モデルへ自動で切り替える（[ADR 002](docs/adr/002-ai-api-gemini-for-development.md) / [ADR 013](docs/adr/013-gemini-finalized-for-production.md) / [ADR 025](docs/adr/025-gemini-model-fallback.md)） |
| 天候 | OpenWeatherMap API | 無料枠が大きい |
| マイグレーション | golang-migrate | up/downをSQLで管理（[ADR 006](docs/adr/006-migration-tool-golang-migrate.md)） |
| デプロイ | Vercel (FE) / Railway (BE) / Supabase (DB) | すべて無料枠中心で運用 |
| CI | GitHub Actions | push/PR時に`go build`・`go vet`・lint・buildを自動実行 |

## システム構成

```
[ブラウザ]
   │
   ▼
[React + TypeScript (Vercel)]
   │  REST API (HTTPS, JWT)
   ▼
[Go + Echo (Railway)]
   │            │              │
   ▼            ▼              ▼
[PostgreSQL  [Gemini API]  [OpenWeatherMap API]
 (Supabase)]
```

フロントエンドはGoバックエンドとのみ通信する。AI・天候APIの呼び出しはすべてservice層に
閉じ込めており、APIキーをフロントに露出させず、AI呼び出し頻度もバックエンドで一元管理する。

バックエンドは `handler`（HTTP受付） → `service`（ビジネスロジック） → `repository`（DBアクセス）の
3層構成。個人開発のMVPとして、厳密なDDDやマイクロサービス化は意図的に採用していない。

## AI提案の仕組み（このアプリの中核）

`GET /advices/latest` が、取得だけでなく生成判定・生成処理まで担う。

1. 記録が3件未満なら、AIを呼ばずに「まず記録を増やしてください」というガイダンスを返す
   （根拠のない一般論を返さないためのコールドスタート対応）
2. 有効な目標が無ければ、目標設定への導線を返す
3. 前回生成から24時間未満なら、DBに保存済みの提案をそのまま返す（キャッシュ）
4. 24時間以上経過していれば、天候取得 → Gemini API呼び出し → DB保存 を行って返す

GETに副作用を持たせているのは、「ユーザーが実際に提案を見にきたときだけ生成する」ことで
API呼び出し回数を最小化するための意図的なトレードオフである
（→ [ADR 004](docs/adr/004-advices-latest-side-effect.md)）。

### 提案の質を上げるための工夫

- **出力をJSONに固定**: `responseMimeType: application/json` を指定し、生成結果を
  そのままDBのjsonb列に保存できるようにした
- **プロバイダの抽象化**: `AdviceGenerator` / `WeatherClient` インターフェースに依存させ、
  AI・天候のプロバイダを差し替えてもservice層以降に影響しない構成にした
  （→ [ADR 012](docs/adr/012-advice-generation-provider-abstraction-and-weather-optional.md)）
- **提案の画一化への対処**: 実データで5回連続して同じペース走が提案される問題が発生した。
  原因を「AIが自分の過去の提案を知らない」「今日の日付・残日数を知らない」「スキーマが
  単一ペースしか表現できない」の3点に切り分け、①直近3回の提案 ②今日の日付とレースまでの
  残日数 をプロンプトに渡し、③`next_menu`にメニュー種別と区間配列（`segments`）を追加した
  （→ [ADR 021](docs/adr/021-menu-diversity-phased-rollout.md) /
  [ADR 022](docs/adr/022-next-menu-segments-and-build-up-type.md)）
- **APIキー漏えい対策**: 外部API失敗時のエラーはURLやレスポンス本文を含みうるため、
  クライアントには定型メッセージのみ返し、詳細はサーバーログにのみ残す。APIキー自体も
  URLのクエリではなくヘッダーで渡し、エラー文にキーが混ざる経路をなくした
- **AIの混雑・上限超過への対処**: 本番で使用モデルが503（混雑）を返し続け、提案が止まる障害が
  起きた。原因をログとローカル再現で切り分け（コード・インフラではなくモデル側の問題）、
  優先順位付きのモデル一覧を順に試すフォールバックを実装した。無料枠のまま可用性を上げる
  選択で、切り替える失敗の種類（混雑・上限超過・廃止）と切り替えない種類（不正リクエスト・認証）も
  区別している（→ [ADR 025](docs/adr/025-gemini-model-fallback.md)）
- **AI出力の検証**: プロンプトで指示した列挙値や区間距離の合計が守られていない場合も、
  提案自体は止めずログに記録して傾向を追えるようにした（jsonbは値を制約しないため）

### 画面の使い勝手

- **エラー時もその場で再試行できる**: 取得に失敗してもボタンごと消えず、エラー文と
  「再読み込み」を出す
- **AI生成の待ち時間を伝える**: 新規生成は数十秒かかることがあるため、一定時間を超えたら
  「新しい提案を生成しています」に表示を切り替える（→ [ADR 024](docs/adr/024-advice-waiting-ux-by-elapsed-time.md)）
- **エラー文はすべて日本語の定型文**: 外部サービスの英語メッセージや内部情報を画面に出さない
  （→ [ADR 023](docs/adr/023-user-facing-error-messages.md)）

## ディレクトリ構成

```
backend/
  cmd/api/          エントリポイント・DI・ルーティング
  internal/
    config/         環境変数の読み込み
    middleware/     JWT検証
    handler/        HTTPリクエスト受付・レスポンス整形
    service/        ビジネスロジック（AI呼び出し・天候取得・進捗計算）
    repository/     DBアクセス
    model/          ドメインモデル
  migrations/       golang-migrateのSQL
frontend/
  src/
    components/     画面・UIコンポーネント
    hooks/          セッション管理
    lib/            APIクライアント・Supabaseクライアント・エラー文変換・日付整形
docs/               要件・アーキテクチャ・DB・API・実装計画
docs/adr/           設計判断の記録（Architecture Decision Record）
```

## ローカルでの起動

前提: Go 1.27+ / Node.js 22 / Docker / [golang-migrate](https://github.com/golang-migrate/migrate) CLI

```bash
# 1. DBを起動
docker compose up -d

# 2. 環境変数を用意（Supabase・Gemini・OpenWeatherMapのキーを設定する）
cp backend/.env.example backend/.env
cp frontend/.env.example frontend/.env

# 3. マイグレーションを適用
migrate -path backend/migrations -database "postgres://pacely:pacely_local_pw@localhost:5432/pacely?sslmode=disable" up

# 4. バックエンド起動（http://localhost:8080）
cd backend && go run ./cmd/api

# 5. フロントエンド起動（http://localhost:5173）
cd frontend && npm install && npm run dev
```

## ドキュメント

設計の意図をコードの外に残すことを重視しており、仕様と設計判断をすべて文書化している。

| ドキュメント | 内容 |
| --- | --- |
| [docs/requirements.md](docs/requirements.md) | 要件定義・MVPスコープ・スコープ外の明確化 |
| [docs/architecture.md](docs/architecture.md) | 技術選定理由・レイヤー構成・AI提案生成フロー |
| [docs/database.md](docs/database.md) | テーブル定義 |
| [docs/api.md](docs/api.md) | APIエンドポイント設計 |
| [docs/implementation-plan.md](docs/implementation-plan.md) | 実装フェーズと進捗 |
| [docs/deployment.md](docs/deployment.md) | デプロイ手順 |
| [docs/devlog.md](docs/devlog.md) | 開発ログ・詰まった点の記録 |
| [docs/adr/](docs/adr/) | 設計判断の記録（26件）。「なぜその選択をしたか」を検討した選択肢とともに残している |

## 開発の進め方

- 1機能ごとにバックエンド+フロントエンドをセットで完成させ、フェーズ単位で動作確認する
- 技術選定・設計判断を行った時点で、背景・検討した選択肢・決定・理由をADRとして残す
- コード変更は`feature/xxx`ブランチで作業し、PRに変更理由を書いてからマージする
- マージ前にCI（build・vet・lint）とコードレビューを通す
