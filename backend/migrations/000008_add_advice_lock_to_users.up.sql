-- docs/adr/031: AI提案の二重生成を防ぐ「生成リース」。
-- GET /advices/latestが生成を始める前に、この列を条件付きUPDATEで「今から3分間は自分が
-- 生成する」と書き込み、書き込めたリクエストだけがAIを呼ぶ。期限付きなので、生成の途中で
-- サーバーが落ちても権利が永久に残らない。
-- NULLは「誰も生成していない」。既存の行・旧コードに影響しないようNULL許容にする
-- (本番はこのマイグレーションを先に適用してからコードをデプロイする)。
ALTER TABLE users ADD COLUMN advice_lock_until timestamptz;
