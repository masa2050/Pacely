# CLAUDE.md

このファイルはClaude Codeがセッション開始時に自動で読み込む、Pacelyプロジェクトの開発ガイドです。

## プロジェクト概要

Pacelyは、ランニング記録に対して「目標に向けて次に何をすべきか」をAIが提案するWebアプリです。記録アプリではなく**AIコーチアプリ**であることが前提。AI提案機能(advices)がプロダクトの中核であり、この機能を軽視した実装をしない。

詳細仕様は以下を必ず参照すること:

- `docs/requirements.md` — 要件定義・MVPスコープ・コールドスタート対応方針
- `docs/architecture.md` — システム構成・技術選定理由・AIアドバイス生成フロー
- `docs/database.md` — テーブル定義
- `docs/api.md` — APIエンドポイント設計(特にGET /advices/latestの処理フローは必読)
- `docs/implementation-plan.md` — 実装フェーズと順序

## 技術スタック

- FE: React + TypeScript (Vite)
- BE: Go + Echo
- DB: PostgreSQL (Supabase)
- 認証: Supabase Auth (JWT)
- AI: Claude API
- 天候: OpenWeatherMap API
- デプロイ: Vercel(FE) / Railway(BE) / Supabase(DB)

## 開発方針(重要)

1. **一度に全部実装しない**。`docs/implementation-plan.md`のフェーズ単位、さらにその中のタスク単位で小さく実装・動作確認する
2. **ユーザー(開発者本人)はWeb開発学習中の大学生**。実装した内容・設計判断は必ず理由とともに説明すること。「AIが全部作って理解していない」状態を避けたいという明確な要望がある
3. 各タスク完了時、以下を簡潔に報告する:
   - 何を作ったか
   - なぜそうしたか
   - どう動いているか
   - 次に何をするか
4. **過剰設計をしない**。個人開発のMVPなので、マイクロサービス化・厳密なDDD・過剰な抽象化は避ける。`docs/architecture.md`のレイヤー構成(handler → service → repository)を守る
5. `GET /advices/latest`はGETだが意図的に副作用(AI呼び出し・DB書き込み)を持つ設計になっている。これは`docs/api.md`に理由を明記した意図的な判断であり、「RESTfulでない」という理由だけで勝手に設計変更しないこと
6. AI API・天候APIの呼び出しはバックエンドのservice層に閉じ込め、フロントエンドから直接外部APIを呼び出さない
7. マイグレーションやテーブル変更を行う場合は、必ず`docs/database.md`の内容と整合性を保つ。スキーマを変更したら`docs/database.md`も更新する

## コーディング規約(最低限)

- Goは標準的なプロジェクトレイアウト(cmd/, internal/など)に従う
- エラーハンドリングを省略しない(握りつぶさない)
- 環境変数(APIキー等)は`.env`で管理し、リポジトリにコミットしない

## やらないこと(スコープ外)

`docs/requirements.md`の「MVPでは実装しない機能」を参照。特に以下は明示的にスコープ外:

- GPSによる自動計測
- Strava/Garmin等の外部サービス連携
- ソーシャル機能
- プッシュ通知
- 複数目標の同時管理
