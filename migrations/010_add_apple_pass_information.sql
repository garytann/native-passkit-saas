ALTER TABLE apple_pass_designs
    ADD COLUMN IF NOT EXISTS stamp_instructions TEXT NOT NULL
        DEFAULT 'Present this pass when making an eligible purchase to collect a stamp.',
    ADD COLUMN IF NOT EXISTS reward_details TEXT NOT NULL
        DEFAULT 'Complete your stamp card to receive the merchant reward.';

CREATE TABLE IF NOT EXISTS apple_pass_links (
    link_id TEXT PRIMARY KEY,
    design_id TEXT NOT NULL,
    link_type TEXT NOT NULL
        CHECK (link_type IN ('website', 'instagram', 'whatsapp', 'custom')),
    label TEXT NOT NULL,
    url TEXT NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0
        CHECK (sort_order BETWEEN 0 AND 20),
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'disabled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_apple_pass_links_design
        FOREIGN KEY (design_id)
        REFERENCES apple_pass_designs (design_id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,
    CONSTRAINT uq_apple_pass_links_design_type
        UNIQUE (design_id, link_type)
);

CREATE INDEX IF NOT EXISTS idx_apple_pass_links_design
    ON apple_pass_links (design_id, status, sort_order);
