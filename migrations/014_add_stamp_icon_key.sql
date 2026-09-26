ALTER TABLE apple_pass_designs
    ADD COLUMN IF NOT EXISTS stamp_icon_key TEXT NOT NULL DEFAULT 'none';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'apple_pass_designs_stamp_icon_key_check'
    ) THEN
        ALTER TABLE apple_pass_designs
            ADD CONSTRAINT apple_pass_designs_stamp_icon_key_check
            CHECK (stamp_icon_key IN ('none', 'coffee', 'star', 'gift', 'heart', 'shopping_bag', 'custom'));
    END IF;
END $$;
