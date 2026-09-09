DO $$
BEGIN
    ALTER TABLE apple_signing_certificates
        DROP CONSTRAINT IF EXISTS apple_signing_certificates_status_check;

    ALTER TABLE apple_signing_certificates
        ADD CONSTRAINT apple_signing_certificates_status_check
        CHECK (status IN ('pending', 'validated', 'active', 'retiring', 'expired', 'revoked'));
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;
