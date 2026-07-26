CREATE TABLE IF NOT EXISTS wallet_passes (
    pass_type_identifier TEXT NOT NULL,
    serial_number TEXT NOT NULL,
    authentication_token TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT wallet_passes_pkey
        PRIMARY KEY (pass_type_identifier, serial_number),
    CONSTRAINT fk_wallet_passes_member
        FOREIGN KEY (serial_number)
        REFERENCES store_members (member_id)
        ON UPDATE CASCADE
        ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_wallet_passes_serial_number
    ON wallet_passes (serial_number);

CREATE INDEX IF NOT EXISTS idx_wallet_passes_last_updated_at
    ON wallet_passes (last_updated_at);
