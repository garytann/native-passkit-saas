CREATE TABLE IF NOT EXISTS apple_wallet_issuers (
    issuer_id TEXT PRIMARY KEY,
    merchant_id TEXT NOT NULL UNIQUE,
    ownership_mode TEXT NOT NULL DEFAULT 'platform_managed'
        CHECK (ownership_mode IN ('platform_managed', 'merchant_managed')),
    team_identifier TEXT NOT NULL DEFAULT '',
    organization_name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'active', 'suspended')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_apple_wallet_issuers_merchant
        FOREIGN KEY (merchant_id)
        REFERENCES merchants (merchant_id)
        ON UPDATE CASCADE
        ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_apple_wallet_issuers_status
    ON apple_wallet_issuers (status);

CREATE TABLE IF NOT EXISTS apple_pass_types (
    apple_pass_type_id TEXT PRIMARY KEY,
    issuer_id TEXT NOT NULL,
    pass_type_identifier TEXT NOT NULL UNIQUE,
    pass_style TEXT NOT NULL DEFAULT 'storeCard'
        CHECK (pass_style IN ('storeCard', 'coupon', 'generic')),
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'active', 'disabled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_apple_pass_types_issuer
        FOREIGN KEY (issuer_id)
        REFERENCES apple_wallet_issuers (issuer_id)
        ON UPDATE CASCADE
        ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_apple_pass_types_issuer
    ON apple_pass_types (issuer_id);

CREATE INDEX IF NOT EXISTS idx_apple_pass_types_status
    ON apple_pass_types (status);

CREATE TABLE IF NOT EXISTS apple_signing_certificates (
    certificate_id TEXT PRIMARY KEY,
    apple_pass_type_id TEXT NOT NULL,
    certificate_ref TEXT NOT NULL,
    private_key_ref TEXT NOT NULL,
    certificate_fingerprint TEXT NOT NULL UNIQUE,
    certificate_serial_number TEXT NOT NULL,
    valid_from TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'active', 'retiring', 'expired', 'revoked')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    activated_at TIMESTAMPTZ,
    CONSTRAINT fk_apple_signing_certificates_pass_type
        FOREIGN KEY (apple_pass_type_id)
        REFERENCES apple_pass_types (apple_pass_type_id)
        ON UPDATE CASCADE
        ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_apple_signing_certificates_pass_type
    ON apple_signing_certificates (apple_pass_type_id);

CREATE INDEX IF NOT EXISTS idx_apple_signing_certificates_expires_at
    ON apple_signing_certificates (expires_at);

CREATE UNIQUE INDEX IF NOT EXISTS idx_apple_signing_certificates_one_active
    ON apple_signing_certificates (apple_pass_type_id)
    WHERE status = 'active';
