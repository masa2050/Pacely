# ADR 030: 本番のDB接続にSupabaseのSession poolerを使い、Transactionモードを避ける

## ステータス
決定済み(事後記録)

この判断はフェーズ6(本番デプロイ)で行い、`docs/deployment.md`の手順の中に注意書きとして
書いていたが、ADRとして残していなかった。

## 背景
本番のBE(Railway)からSupabaseのPostgreSQLへ接続する際、Supabaseのダッシュボードには
接続方法が複数用意されていた。

- **Direct connection**: PostgreSQLに直接つなぐ(`db.<プロジェクトID>.supabase.co:5432`)
- **Session pooler**: コネクションプーラー経由で、1クライアントが接続している間は
  同じDB接続を使い続ける(`<リージョン>.pooler.supabase.com:5432`)
- **Transaction pooler**: コネクションプーラー経由で、トランザクションごとに別のDB接続を
  割り当てる(ポート`6543`)

BEはDBドライバに`pgx`(`pgxpool`)を使っており、`pgx`はデフォルトでプリペアドステートメント
(SQLを先にDBへ登録し、2回目以降は登録済みのものを使う仕組み)を使う。

## 検討した選択肢
- **A: Transaction pooler**: 同時接続数が多いサーバーレス環境向け。ただしトランザクションごとに
  DB接続が変わるため、ある接続に登録したプリペアドステートメントが次のクエリでは存在せず、
  `pgx`のデフォルト設定のままでは実行時エラーになる
- **B: Transaction pooler + `pgx`のプリペアドステートメントを無効化する**: 接続文字列や
  `pgx`の設定で、プリペアドステートメントを使わない実行モードに切り替える。動作はするが、
  Transactionモードを選ぶ必要性(大量の同時接続)が、MVPの規模には無い
- **C: Session pooler(採用)**: 接続している間は同じDB接続が使われるため、プリペアドステートメントが
  そのまま動く。BEのコードや`pgx`の設定を変える必要が無い
- **D: Direct connection**: プーラーを経由しないので、プリペアドステートメントの問題も起きない。
  ただしSupabaseのDirect connectionは、IPv4での接続に追加のオプションが必要になる場合がある。
  なお、当時はDirectとSessionを詳しく比べてはおらず、`docs/deployment.md`の手順どおりSession poolerを
  選んでいた。この項目は事後記録の際に整理したもの

## 決定
本番の`DATABASE_URL`には、Session poolerの接続文字列を使う。

## 理由
1. RailwayのBEは常駐サーバーで、MVPの規模では同時接続数も少ないため、Transactionモードを選ぶ
   必要が無い
2. `pgx`のデフォルト(プリペアドステートメント)のまま動かせるため、BEのコードに手を入れずに済む
3. プーラー経由でも、BE側の`pgxpool`で接続を使い回すため、性能面の不利はMVPでは問題にならない

## 影響・トレードオフ
- `docs/deployment.md`の手順では、Transactionモードを使わないよう注意書きを残している
- 2026-10-25の障害(`docs/devlog.md`)では、Session poolerが、古いパスワードによる認証失敗の繰り返しを
  検知して新規接続を一時ブロックした(`ECIRCUITBREAKER`)。プーラーを経由する構成では、
  DB本体とは別に、プーラー側の保護機能の挙動も把握しておく必要がある
- 将来、サーバーレス環境に移すなど同時接続数が増える構成に変える場合は、Bを再検討する
