CREATE TABLE IF NOT EXISTS loyalty_programs (
    program_id TEXT PRIMARY KEY,
    merchant_id TEXT NOT NULL UNIQUE,
    program_name TEXT NOT NULL,
    pass_description TEXT NOT NULL,
    stamp_goal INTEGER NOT NULL DEFAULT 10 CHECK (stamp_goal BETWEEN 1 AND 100),
    background_color TEXT NOT NULL DEFAULT '#5434F5',
    asset_bundle_ref TEXT NOT NULL DEFAULT '',
    stamp_instructions TEXT NOT NULL,
    reward_title TEXT NOT NULL CHECK (CHAR_LENGTH(reward_title) BETWEEN 1 AND 40),
    reward_details TEXT NOT NULL,
    support_text TEXT NOT NULL,
    terms_text TEXT NOT NULL,
    campaign_display_mode TEXT NOT NULL DEFAULT 'details'
        CHECK (campaign_display_mode IN ('none', 'details', 'front_and_details')),
    stamp_icon_key TEXT NOT NULL DEFAULT 'logo'
        CHECK (stamp_icon_key IN ('none', 'logo', 'coffee', 'star', 'gift', 'heart', 'shopping_bag', 'custom')),
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'active', 'disabled')),
    version INTEGER NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_loyalty_programs_merchant
        FOREIGN KEY (merchant_id) REFERENCES merchants (merchant_id)
        ON UPDATE CASCADE ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_loyalty_programs_status ON loyalty_programs (status);

CREATE TABLE IF NOT EXISTS loyalty_program_links (
    link_id TEXT PRIMARY KEY,
    program_id TEXT NOT NULL,
    link_type TEXT NOT NULL CHECK (link_type IN ('website', 'instagram', 'whatsapp', 'custom')),
    label TEXT NOT NULL,
    url TEXT NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0 CHECK (sort_order BETWEEN 0 AND 20),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_loyalty_program_links_program
        FOREIGN KEY (program_id) REFERENCES loyalty_programs (program_id)
        ON UPDATE CASCADE ON DELETE CASCADE,
    CONSTRAINT uq_loyalty_program_links_type UNIQUE (program_id, link_type)
);

CREATE TABLE IF NOT EXISTS apple_pass_programs (
    apple_pass_type_id TEXT PRIMARY KEY,
    program_id TEXT NOT NULL,
    foreground_color TEXT NOT NULL DEFAULT '#FFFFFF',
    label_color TEXT NOT NULL DEFAULT '#E1DCFF',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_apple_pass_programs_pass_type
        FOREIGN KEY (apple_pass_type_id) REFERENCES apple_pass_types (apple_pass_type_id)
        ON UPDATE CASCADE ON DELETE CASCADE,
    CONSTRAINT fk_apple_pass_programs_program
        FOREIGN KEY (program_id) REFERENCES loyalty_programs (program_id)
        ON UPDATE CASCADE ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_apple_pass_programs_program ON apple_pass_programs (program_id);

WITH latest_design AS (
    SELECT DISTINCT ON (awi.merchant_id)
        design.*, awi.merchant_id
    FROM apple_pass_designs design
    INNER JOIN apple_pass_types apt ON apt.apple_pass_type_id = design.apple_pass_type_id
    INNER JOIN apple_wallet_issuers awi ON awi.issuer_id = apt.issuer_id
    ORDER BY awi.merchant_id, design.updated_at DESC, design.created_at DESC
)
INSERT INTO loyalty_programs (
    program_id, merchant_id, program_name, pass_description, stamp_goal,
    background_color, asset_bundle_ref, stamp_instructions, reward_title,
    reward_details, support_text, terms_text, campaign_display_mode,
    stamp_icon_key, status, version, created_at, updated_at
)
SELECT
    design_id, merchant_id, program_name, pass_description, stamp_goal,
    background_color, asset_bundle_ref, stamp_instructions, reward_title,
    reward_details, support_text, terms_text, campaign_display_mode,
    stamp_icon_key, status, version, created_at, updated_at
FROM latest_design
ON CONFLICT (merchant_id) DO NOTHING;

INSERT INTO loyalty_program_links (
    link_id, program_id, link_type, label, url, sort_order, status, created_at, updated_at
)
SELECT link.link_id, program.program_id, link.link_type, link.label, link.url,
       link.sort_order, link.status, link.created_at, link.updated_at
FROM apple_pass_links link
INNER JOIN apple_pass_designs design ON design.design_id = link.design_id
INNER JOIN apple_pass_types apt ON apt.apple_pass_type_id = design.apple_pass_type_id
INNER JOIN apple_wallet_issuers awi ON awi.issuer_id = apt.issuer_id
INNER JOIN loyalty_programs program ON program.merchant_id = awi.merchant_id
ON CONFLICT (program_id, link_type) DO NOTHING;

INSERT INTO apple_pass_programs (
    apple_pass_type_id, program_id, foreground_color, label_color, created_at, updated_at
)
SELECT design.apple_pass_type_id, program.program_id, design.foreground_color,
       design.label_color, design.created_at, design.updated_at
FROM apple_pass_designs design
INNER JOIN apple_pass_types apt ON apt.apple_pass_type_id = design.apple_pass_type_id
INNER JOIN apple_wallet_issuers awi ON awi.issuer_id = apt.issuer_id
INNER JOIN loyalty_programs program ON program.merchant_id = awi.merchant_id
ON CONFLICT (apple_pass_type_id) DO NOTHING;

CREATE TABLE IF NOT EXISTS google_wallet_classes (
    google_class_id TEXT PRIMARY KEY,
    program_id TEXT NOT NULL UNIQUE,
    issuer_id TEXT NOT NULL,
    class_suffix TEXT NOT NULL,
    review_status TEXT NOT NULL DEFAULT 'DRAFT'
        CHECK (review_status IN ('DRAFT', 'UNDER_REVIEW', 'APPROVED', 'REJECTED')),
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'active', 'disabled', 'error')),
    program_logo_ref TEXT NOT NULL DEFAULT '',
    wide_logo_ref TEXT NOT NULL DEFAULT '',
    synced_program_version INTEGER NOT NULL DEFAULT 0,
    last_synced_at TIMESTAMPTZ,
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_google_wallet_classes_program
        FOREIGN KEY (program_id) REFERENCES loyalty_programs (program_id)
        ON UPDATE CASCADE ON DELETE CASCADE,
    CONSTRAINT uq_google_wallet_class_suffix UNIQUE (issuer_id, class_suffix)
);

CREATE INDEX IF NOT EXISTS idx_google_wallet_classes_status
    ON google_wallet_classes (status, review_status);

CREATE TABLE IF NOT EXISTS google_wallet_objects (
    google_object_id TEXT PRIMARY KEY,
    member_id TEXT NOT NULL UNIQUE,
    google_class_id TEXT NOT NULL,
    image_token TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT 'provisioned'
        CHECK (status IN ('provisioned', 'active', 'disabled', 'sync_error')),
    synced_stamp_progress INTEGER NOT NULL DEFAULT 0,
    synced_available_rewards INTEGER NOT NULL DEFAULT 0,
    synced_program_version INTEGER NOT NULL DEFAULT 0,
    last_synced_at TIMESTAMPTZ,
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_google_wallet_objects_member
        FOREIGN KEY (member_id) REFERENCES store_members (member_id)
        ON UPDATE CASCADE ON DELETE CASCADE,
    CONSTRAINT fk_google_wallet_objects_class
        FOREIGN KEY (google_class_id) REFERENCES google_wallet_classes (google_class_id)
        ON UPDATE CASCADE ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_google_wallet_objects_class
    ON google_wallet_objects (google_class_id);

CREATE TABLE IF NOT EXISTS wallet_sync_jobs (
    sync_job_id TEXT PRIMARY KEY,
    platform TEXT NOT NULL CHECK (platform IN ('apple', 'google')),
    target_type TEXT NOT NULL CHECK (target_type IN ('class', 'object', 'campaign')),
    target_id TEXT NOT NULL,
    desired_version TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    max_attempts INTEGER NOT NULL DEFAULT 8 CHECK (max_attempts > 0),
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    CONSTRAINT uq_wallet_sync_job_version
        UNIQUE (platform, target_type, target_id, desired_version)
);

CREATE INDEX IF NOT EXISTS idx_wallet_sync_jobs_poll
    ON wallet_sync_jobs (status, next_attempt_at);

CREATE OR REPLACE FUNCTION sync_apple_design_to_loyalty_program()
RETURNS TRIGGER AS $$
DECLARE
    target_merchant_id TEXT;
    target_program_id TEXT;
BEGIN
    SELECT awi.merchant_id INTO target_merchant_id
    FROM apple_pass_types apt
    INNER JOIN apple_wallet_issuers awi ON awi.issuer_id = apt.issuer_id
    WHERE apt.apple_pass_type_id = NEW.apple_pass_type_id;

    INSERT INTO loyalty_programs (
        program_id, merchant_id, program_name, pass_description, stamp_goal,
        background_color, asset_bundle_ref, stamp_instructions, reward_title,
        reward_details, support_text, terms_text, campaign_display_mode,
        stamp_icon_key, status, version, created_at, updated_at
    ) VALUES (
        NEW.design_id, target_merchant_id, NEW.program_name, NEW.pass_description,
        NEW.stamp_goal, NEW.background_color, NEW.asset_bundle_ref,
        NEW.stamp_instructions, NEW.reward_title, NEW.reward_details,
        NEW.support_text, NEW.terms_text, NEW.campaign_display_mode,
        NEW.stamp_icon_key, NEW.status, NEW.version, NEW.created_at, NEW.updated_at
    )
    ON CONFLICT (merchant_id) DO UPDATE SET
        program_name = EXCLUDED.program_name,
        pass_description = EXCLUDED.pass_description,
        stamp_goal = EXCLUDED.stamp_goal,
        background_color = EXCLUDED.background_color,
        asset_bundle_ref = EXCLUDED.asset_bundle_ref,
        stamp_instructions = EXCLUDED.stamp_instructions,
        reward_title = EXCLUDED.reward_title,
        reward_details = EXCLUDED.reward_details,
        support_text = EXCLUDED.support_text,
        terms_text = EXCLUDED.terms_text,
        campaign_display_mode = EXCLUDED.campaign_display_mode,
        stamp_icon_key = EXCLUDED.stamp_icon_key,
        status = EXCLUDED.status,
        version = EXCLUDED.version,
        updated_at = EXCLUDED.updated_at;

    SELECT program_id INTO target_program_id
    FROM loyalty_programs WHERE merchant_id = target_merchant_id;

    INSERT INTO apple_pass_programs (
        apple_pass_type_id, program_id, foreground_color, label_color, created_at, updated_at
    ) VALUES (
        NEW.apple_pass_type_id, target_program_id, NEW.foreground_color,
        NEW.label_color, NEW.created_at, NEW.updated_at
    )
    ON CONFLICT (apple_pass_type_id) DO UPDATE SET
        program_id = EXCLUDED.program_id,
        foreground_color = EXCLUDED.foreground_color,
        label_color = EXCLUDED.label_color,
        updated_at = EXCLUDED.updated_at;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_sync_apple_design_to_loyalty_program ON apple_pass_designs;
CREATE TRIGGER trg_sync_apple_design_to_loyalty_program
AFTER INSERT OR UPDATE ON apple_pass_designs
FOR EACH ROW EXECUTE FUNCTION sync_apple_design_to_loyalty_program();

CREATE OR REPLACE FUNCTION sync_apple_link_to_loyalty_program()
RETURNS TRIGGER AS $$
DECLARE
    target_program_id TEXT;
BEGIN
    SELECT program.program_id INTO target_program_id
    FROM apple_pass_designs design
    INNER JOIN apple_pass_types apt ON apt.apple_pass_type_id = design.apple_pass_type_id
    INNER JOIN apple_wallet_issuers awi ON awi.issuer_id = apt.issuer_id
    INNER JOIN loyalty_programs program ON program.merchant_id = awi.merchant_id
    WHERE design.design_id = NEW.design_id;

    INSERT INTO loyalty_program_links (
        link_id, program_id, link_type, label, url, sort_order, status, created_at, updated_at
    ) VALUES (
        NEW.link_id, target_program_id, NEW.link_type, NEW.label, NEW.url,
        NEW.sort_order, NEW.status, NEW.created_at, NEW.updated_at
    )
    ON CONFLICT (program_id, link_type) DO UPDATE SET
        link_id = EXCLUDED.link_id,
        label = EXCLUDED.label,
        url = EXCLUDED.url,
        sort_order = EXCLUDED.sort_order,
        status = EXCLUDED.status,
        updated_at = EXCLUDED.updated_at;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_sync_apple_link_to_loyalty_program ON apple_pass_links;
CREATE TRIGGER trg_sync_apple_link_to_loyalty_program
AFTER INSERT OR UPDATE ON apple_pass_links
FOR EACH ROW EXECUTE FUNCTION sync_apple_link_to_loyalty_program();

CREATE OR REPLACE FUNCTION delete_apple_link_from_loyalty_program()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM loyalty_program_links WHERE link_id = OLD.link_id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_delete_apple_link_from_loyalty_program ON apple_pass_links;
CREATE TRIGGER trg_delete_apple_link_from_loyalty_program
AFTER DELETE ON apple_pass_links
FOR EACH ROW EXECUTE FUNCTION delete_apple_link_from_loyalty_program();
