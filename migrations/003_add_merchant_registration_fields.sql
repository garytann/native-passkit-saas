ALTER TABLE merchants
    ADD COLUMN IF NOT EXISTS first_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS last_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS password_hash TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS payment_status TEXT NOT NULL DEFAULT 'inactive';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'merchants_payment_status_check'
    ) THEN
        ALTER TABLE merchants
            ADD CONSTRAINT merchants_payment_status_check
            CHECK (payment_status IN ('active', 'inactive'));
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS idx_merchants_merchant_email_unique
    ON merchants (merchant_email);
