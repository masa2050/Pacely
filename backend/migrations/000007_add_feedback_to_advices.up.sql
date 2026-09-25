-- フェーズ7-5: AIアドバイスへのフィードバック(docs/adr/019)。
--
-- アドバイスとフィードバックの関係は厳密に1:1(advices1件は必ず1ユーザーのもので、
-- 他人が評価することはない)ため、テーブルを分けずadvicesのカラムとして持つ。
-- 3カラムともNULL許容 = 未評価。既存行はすべて未評価として扱われる。
-- is_helpfulはbooleanにしている: 現時点の要件が「役に立った/役に立たなかった」の
-- 2値のみで、段階評価(smallint)や種別(varchar)は今使わない拡張性のため。
ALTER TABLE advices
    ADD COLUMN is_helpful boolean,
    ADD COLUMN feedback_comment text,
    ADD COLUMN feedback_at timestamptz;
