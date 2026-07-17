ALTER TABLE merchants
    ADD COLUMN IF NOT EXISTS merchant_slug TEXT;

UPDATE merchants
SET merchant_slug = LOWER(REGEXP_REPLACE(TRIM(BOTH '-' FROM REGEXP_REPLACE(merchant, '[^a-zA-Z0-9]+', '-', 'g')), '-+', '-', 'g'))
WHERE merchant_slug IS NULL OR merchant_slug = '';

ALTER TABLE merchants
    ALTER COLUMN merchant_slug SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_merchants_merchant_slug_unique
    ON merchants (merchant_slug);
