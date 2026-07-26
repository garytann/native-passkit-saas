CREATE TABLE IF NOT EXISTS campaigns (
    campaign_id TEXT PRIMARY KEY,
    creation_date TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    merchant TEXT NOT NULL,
    title TEXT NOT NULL,
    message TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'scheduled', 'active', 'sent', 'cancelled')),
    start_date TIMESTAMPTZ,
    end_date TIMESTAMPTZ,
    sent_at TIMESTAMPTZ,
    CONSTRAINT fk_campaigns_merchant
        FOREIGN KEY (merchant)
        REFERENCES merchants (merchant)
        ON UPDATE CASCADE
        ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_campaigns_merchant
    ON campaigns (merchant);

CREATE INDEX IF NOT EXISTS idx_campaigns_status
    ON campaigns (status);

CREATE TABLE IF NOT EXISTS campaign_deliveries (
    delivery_id TEXT PRIMARY KEY,
    creation_date TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    campaign_id TEXT NOT NULL,
    member_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'queued', 'sent', 'failed', 'redeemed')),
    delivered_at TIMESTAMPTZ,
    viewed_at TIMESTAMPTZ,
    redeemed_at TIMESTAMPTZ,
    last_error TEXT NOT NULL DEFAULT '',
    CONSTRAINT fk_campaign_deliveries_campaign
        FOREIGN KEY (campaign_id)
        REFERENCES campaigns (campaign_id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,
    CONSTRAINT fk_campaign_deliveries_member
        FOREIGN KEY (member_id)
        REFERENCES store_members (member_id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,
    CONSTRAINT campaign_deliveries_campaign_member_unique
        UNIQUE (campaign_id, member_id)
);

CREATE INDEX IF NOT EXISTS idx_campaign_deliveries_campaign
    ON campaign_deliveries (campaign_id);

CREATE INDEX IF NOT EXISTS idx_campaign_deliveries_member
    ON campaign_deliveries (member_id);

CREATE INDEX IF NOT EXISTS idx_campaign_deliveries_status
    ON campaign_deliveries (status);

CREATE TABLE IF NOT EXISTS wallet_registrations (
    device_library_identifier TEXT NOT NULL,
    pass_type_identifier TEXT NOT NULL,
    serial_number TEXT NOT NULL,
    push_token TEXT NOT NULL,
    authentication_token TEXT NOT NULL,
    registered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT wallet_registrations_pkey
        PRIMARY KEY (device_library_identifier, pass_type_identifier, serial_number),
    CONSTRAINT fk_wallet_registrations_member
        FOREIGN KEY (serial_number)
        REFERENCES store_members (member_id)
        ON UPDATE CASCADE
        ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_wallet_registrations_serial_number
    ON wallet_registrations (serial_number);

CREATE INDEX IF NOT EXISTS idx_wallet_registrations_push_token
    ON wallet_registrations (push_token);
