ALTER TABLE apple_pass_designs
    DROP CONSTRAINT IF EXISTS apple_pass_designs_campaign_display_mode_check;

ALTER TABLE apple_pass_designs
    ADD CONSTRAINT apple_pass_designs_campaign_display_mode_check
    CHECK (campaign_display_mode IN ('none', 'details', 'front_and_details'));
