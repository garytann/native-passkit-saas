CREATE TABLE IF NOT EXISTS apple_pass_designs (
    design_id TEXT PRIMARY KEY,
    apple_pass_type_id TEXT NOT NULL UNIQUE,
    program_name TEXT NOT NULL,
    pass_description TEXT NOT NULL,
    stamp_goal INTEGER NOT NULL DEFAULT 10
        CHECK (stamp_goal BETWEEN 1 AND 100),
    background_color TEXT NOT NULL DEFAULT '#5434F5',
    foreground_color TEXT NOT NULL DEFAULT '#FFFFFF',
    label_color TEXT NOT NULL DEFAULT '#E1DCFF',
    asset_bundle_ref TEXT NOT NULL DEFAULT '',
    support_text TEXT NOT NULL DEFAULT 'Contact the merchant for loyalty card assistance.',
    terms_text TEXT NOT NULL DEFAULT 'Collect one stamp for each eligible purchase. Rewards are subject to outlet terms.',
    campaign_display_mode TEXT NOT NULL DEFAULT 'details'
        CHECK (campaign_display_mode IN ('none', 'details')),
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('draft', 'active', 'disabled')),
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_apple_pass_designs_pass_type
        FOREIGN KEY (apple_pass_type_id)
        REFERENCES apple_pass_types (apple_pass_type_id)
        ON UPDATE CASCADE
        ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_apple_pass_designs_status
    ON apple_pass_designs (status);
