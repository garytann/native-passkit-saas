ALTER TABLE apple_pass_designs
    ADD COLUMN IF NOT EXISTS reward_title TEXT NOT NULL
        DEFAULT 'Loyalty Reward';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'apple_pass_designs_reward_title_length_check'
    ) THEN
        ALTER TABLE apple_pass_designs
            ADD CONSTRAINT apple_pass_designs_reward_title_length_check
            CHECK (CHAR_LENGTH(reward_title) BETWEEN 1 AND 40);
    END IF;
END $$;
