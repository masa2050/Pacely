# Pacely データベース設計書

## 1. 設計方針

- 個人開発MVPのため、正規化は最低限に留めつつ、将来の拡張(履歴管理・複数目標対応など)を妨げない構成にする
- 目標(goals)・AI提案(advices)はどちらも**履歴を残す設計**とする(達成済み目標の振り返り、過去のAI提案の変遷を確認できるようにするため)
- 「現在有効な目標」「最新のAI提案」は、is_activeフラグや最新レコード取得で判定する

## 2. ER概要

```
users 1 ── * runs
users 1 ── * goals
users 1 ── * advices
goals  1 ── * advices (どの目標に対する提案か紐付け)
```

## 3. テーブル定義

### 3.1 users

Supabase Authが管理するユーザーテーブルをベースに利用する(認証情報自体はSupabase Auth側が保持)。Pacely独自のプロフィール情報のみ、拡張テーブルとして持つ。

| カラム名   | 型          | 説明                                   |
| ---------- | ----------- | --------------------------------------- |
| id         | uuid (PK)   | Supabase Authのuser idと同一            |
| region     | varchar     | 天候取得用の地域(都道府県/市)          |
| username   | varchar(50) | 画面上の表示名(未設定可、フェーズ7-3) |
| created_at | timestamp   | 作成日時                                |

### 3.2 runs(ランニング記録)

| カラム名        | 型                   | 説明                                                                                                                                      |
| --------------- | -------------------- | ----------------------------------------------------------------------------------------------------------------------------------------- |
| id              | uuid (PK)            |                                                                                                                                           |
| user_id         | uuid (FK → users.id) |                                                                                                                                           |
| distance_km     | numeric              | 距離(km)                                                                                                                                  |
| duration_sec    | integer              | 所要時間(秒)                                                                                                                              |
| pace_sec_per_km | numeric              | ペース(自動計算・保存する。distance/durationから算出するが、毎回計算せず保存しておくことで一覧表示のクエリを軽くする)                     |
| rpe             | integer (nullable)   | 体感的きつさ(Rate of Perceived Exertion)。1〜10で自己申告。AIへの入力材料として使う(同じペースでもきつさが違えば疲労蓄積の判断材料になる) |
| run_date        | date                 | 走った日付                                                                                                                                |
| created_at      | timestamp            | 記録作成日時                                                                                                                              |

### 3.3 goals(目標)

| カラム名        | 型                   | 説明                                 |
| --------------- | -------------------- | ------------------------------------ |
| id              | uuid (PK)            |                                      |
| user_id         | uuid (FK → users.id) |                                      |
| goal_type       | varchar              | 例: "full_marathon", "half_marathon" |
| target_time_sec | integer              | 目標タイム(秒)                       |
| target_date     | date                 | 目標達成予定日(レース日など)         |
| status          | varchar              | "active" / "achieved" / "abandoned"  |
| created_at      | timestamp            |                                      |

- **現在有効な目標**は `status = 'active'` のレコード(基本的に1ユーザーにつき1件のみ運用。バックエンド側のservice層でこの制約を担保する)
- 達成・断念した場合は新規INSERTせず、既存レコードのstatusを更新し、新しい目標は別レコードとしてINSERTする(履歴として残る)

### 3.4 advices(AI提案)

| カラム名        | 型                             | 説明                                                                                             |
| --------------- | ------------------------------ | ------------------------------------------------------------------------------------------------ |
| id              | uuid (PK)                      |                                                                                                  |
| user_id         | uuid (FK → users.id)           |                                                                                                  |
| goal_id         | uuid (FK → goals.id, nullable) | どの目標に対する提案か(目標未設定時はnull)                                                       |
| advice_text     | text                           | AIによるアドバイス本文                                                                           |
| next_menu       | jsonb                          | 次回練習メニュー(構造化データ。例: `{"distance_km": 10, "pace_sec_per_km": 330, "note": "..."}`) |
| weather_context | jsonb                          | 生成時に参照した天候情報(後から「なぜこの提案になったか」を追える)                               |
| generated_at    | timestamp                      | 生成日時(この値をもとに「24時間以内かどうか」を判定する)                                         |

- 最新の提案は `ORDER BY generated_at DESC LIMIT 1` で取得
- 履歴一覧はuser_idで絞り込んで時系列表示する

## 4. インデックス方針

過剰なインデックス設計はせず、検索頻度が高いカラムのみ設定する。

- `runs.user_id`, `runs.run_date` に複合インデックス(記録一覧をユーザー・日付で絞り込むため)
- `goals.user_id`, `goals.status` に複合インデックス(有効な目標を素早く取得するため)
- `advices.user_id`, `advices.generated_at` に複合インデックス(最新提案の取得・履歴表示のため)

## 5. 設計上の注意点

- `pace_sec_per_km` はdistanceとdurationから計算可能な値だが、あえてrunsテーブルに保存する(一覧表示のたびに計算し直すコストを避けるため)。これは正規化を崩す判断だが、読み取り頻度の高さを優先した意図的なトレードオフ
- `next_menu` と `weather_context` をjsonb型にしているのは、AIの出力構造が今後変わる可能性が高く、カラムを都度追加するより柔軟に対応できるようにするため。ただし検索条件には使わない想定(あくまで表示用データ)
- `runs.user_id`・`goals.user_id`・`advices.user_id`・`advices.goal_id`の外部キーは`ON DELETE CASCADE`にしている(migrations/000005)。退会機能(`DELETE /users/me`)で`users`行を削除した際、関連する記録・目標・AI提案がアプリ側で1テーブルずつ削除しなくても自動的に消える(docs/adr/014)
