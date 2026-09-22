ALTER TABLE advices DROP CONSTRAINT advices_goal_id_fkey;
ALTER TABLE advices ADD CONSTRAINT advices_goal_id_fkey
    FOREIGN KEY (goal_id) REFERENCES goals(id);

ALTER TABLE advices DROP CONSTRAINT advices_user_id_fkey;
ALTER TABLE advices ADD CONSTRAINT advices_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id);

ALTER TABLE goals DROP CONSTRAINT goals_user_id_fkey;
ALTER TABLE goals ADD CONSTRAINT goals_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id);

ALTER TABLE runs DROP CONSTRAINT runs_user_id_fkey;
ALTER TABLE runs ADD CONSTRAINT runs_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id);
