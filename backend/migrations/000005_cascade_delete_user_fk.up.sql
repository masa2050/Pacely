-- 退会機能(docs/adr/014)で users テーブルの行を削除したとき、
-- runs/goals/advices の関連行もDB側で自動的に削除されるようにする。
-- アプリ側で子テーブルを1つずつ手動削除するより、削除漏れが起きない。
ALTER TABLE runs DROP CONSTRAINT runs_user_id_fkey;
ALTER TABLE runs ADD CONSTRAINT runs_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE goals DROP CONSTRAINT goals_user_id_fkey;
ALTER TABLE goals ADD CONSTRAINT goals_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE advices DROP CONSTRAINT advices_user_id_fkey;
ALTER TABLE advices ADD CONSTRAINT advices_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

-- goalsがuser削除に連動して消える際、advices.goal_idのFK制約に
-- 引っかからないようにgoal_id側もCASCADEにする。
ALTER TABLE advices DROP CONSTRAINT advices_goal_id_fkey;
ALTER TABLE advices ADD CONSTRAINT advices_goal_id_fkey
    FOREIGN KEY (goal_id) REFERENCES goals(id) ON DELETE CASCADE;
