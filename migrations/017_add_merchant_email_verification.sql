DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'merchants'
          AND column_name = 'email_verified_at'
    ) THEN
        ALTER TABLE merchants
            ADD COLUMN email_verified_at TIMESTAMPTZ;

        UPDATE merchants
        SET email_verified_at = COALESCE(creation_date, NOW())
        WHERE password_hash <> '';
    END IF;
END $$;

ALTER TABLE merchants
    ALTER COLUMN status SET DEFAULT 'pending_verification';

ALTER TABLE merchants
    DROP CONSTRAINT IF EXISTS merchants_status_check;

ALTER TABLE merchants
    ADD CONSTRAINT merchants_status_check
    CHECK (status IN ('pending_verification', 'active', 'inactive'));

CREATE TABLE IF NOT EXISTS merchant_email_verifications (
    verification_id TEXT PRIMARY KEY,
    merchant_id TEXT NOT NULL,
    recipient_email TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    max_attempts INTEGER NOT NULL DEFAULT 5 CHECK (max_attempts > 0),
    resend_available_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_merchant_email_verifications_merchant
        FOREIGN KEY (merchant_id) REFERENCES merchants (merchant_id)
        ON UPDATE CASCADE ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_merchant_email_verifications_lookup
    ON merchant_email_verifications (recipient_email, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_merchant_email_verifications_merchant
    ON merchant_email_verifications (merchant_id, created_at DESC);

CREATE TABLE IF NOT EXISTS email_jobs (
    email_job_id TEXT PRIMARY KEY,
    job_type TEXT NOT NULL
        CHECK (job_type IN ('merchant_verification', 'merchant_welcome')),
    merchant_id TEXT NOT NULL,
    verification_id TEXT,
    recipient_email TEXT NOT NULL,
    recipient_name TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'sent', 'failed')),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    max_attempts INTEGER NOT NULL DEFAULT 8 CHECK (max_attempts > 0),
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_error TEXT NOT NULL DEFAULT '',
    message_id TEXT NOT NULL,
    idempotency_key TEXT NOT NULL UNIQUE,
    sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_email_jobs_merchant
        FOREIGN KEY (merchant_id) REFERENCES merchants (merchant_id)
        ON UPDATE CASCADE ON DELETE CASCADE,
    CONSTRAINT fk_email_jobs_verification
        FOREIGN KEY (verification_id) REFERENCES merchant_email_verifications (verification_id)
        ON UPDATE CASCADE ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_email_jobs_poll
    ON email_jobs (status, next_attempt_at, created_at);

