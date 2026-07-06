CREATE TABLE IF NOT EXISTS store_members (
    member_id TEXT PRIMARY KEY,
    creation_date TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    customer_name TEXT NOT NULL,
    customer_email TEXT NOT NULL,
    merchant TEXT NOT NULL,
    stamp_progress INTEGER NOT NULL DEFAULT 0 CHECK (stamp_progress >= 0),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'reward_ready'))
);

CREATE INDEX IF NOT EXISTS idx_store_members_customer_email
    ON store_members (customer_email);

CREATE INDEX IF NOT EXISTS idx_store_members_merchant
    ON store_members (merchant);
