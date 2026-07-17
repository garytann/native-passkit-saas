CREATE TABLE IF NOT EXISTS merchants (
    merchant_id TEXT PRIMARY KEY,
    creation_date TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    first_name TEXT NOT NULL DEFAULT '',
    last_name TEXT NOT NULL DEFAULT '',
    merchant TEXT NOT NULL UNIQUE,
    merchant_slug TEXT NOT NULL UNIQUE,
    merchant_email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive')),
    payment_status TEXT NOT NULL DEFAULT 'inactive' CHECK (payment_status IN ('active', 'inactive'))
);

CREATE INDEX IF NOT EXISTS idx_merchants_merchant_email
    ON merchants (merchant_email);

-- Backfill merchants from existing member rows so the FK can be introduced
-- without breaking local POC data. Replace placeholder emails later.
INSERT INTO merchants (merchant_id, merchant, merchant_slug, merchant_email)
SELECT
    'MCH-' || UPPER(SUBSTRING(MD5(merchant) FROM 1 FOR 8)),
    merchant,
    LOWER(REGEXP_REPLACE(TRIM(BOTH '-' FROM REGEXP_REPLACE(merchant, '[^a-zA-Z0-9]+', '-', 'g')), '-+', '-', 'g')),
    LOWER(REGEXP_REPLACE(merchant, '[^a-zA-Z0-9]+', '.', 'g')) || '@example.com'
FROM (
    SELECT DISTINCT merchant
    FROM store_members
    WHERE merchant IS NOT NULL AND merchant <> ''
) existing_merchants
ON CONFLICT (merchant) DO NOTHING;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.table_constraints
        WHERE constraint_name = 'fk_store_members_merchant'
          AND table_name = 'store_members'
    ) THEN
        ALTER TABLE store_members
            ADD CONSTRAINT fk_store_members_merchant
            FOREIGN KEY (merchant)
            REFERENCES merchants (merchant)
            ON UPDATE CASCADE
            ON DELETE RESTRICT;
    END IF;
END $$;
