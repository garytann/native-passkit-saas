ALTER TABLE apple_pass_designs
    ALTER COLUMN stamp_icon_key SET DEFAULT 'logo';

ALTER TABLE apple_pass_designs
    DROP CONSTRAINT IF EXISTS apple_pass_designs_stamp_icon_key_check;

ALTER TABLE apple_pass_designs
    ADD CONSTRAINT apple_pass_designs_stamp_icon_key_check
    CHECK (stamp_icon_key IN ('none', 'logo', 'coffee', 'star', 'gift', 'heart', 'shopping_bag', 'custom'));

