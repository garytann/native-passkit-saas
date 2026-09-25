ALTER TABLE store_members
    ADD COLUMN IF NOT EXISTS available_rewards INTEGER NOT NULL DEFAULT 0;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'store_members_available_rewards_check'
    ) THEN
        ALTER TABLE store_members
            ADD CONSTRAINT store_members_available_rewards_check
            CHECK (available_rewards >= 0);
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS member_reward_transactions (
    transaction_id TEXT PRIMARY KEY,
    member_id TEXT NOT NULL REFERENCES store_members(member_id) ON UPDATE CASCADE ON DELETE CASCADE,
    merchant_id TEXT NOT NULL REFERENCES merchants(merchant_id) ON UPDATE CASCADE ON DELETE RESTRICT,
    transaction_type TEXT NOT NULL CHECK (transaction_type IN ('issue', 'redeem')),
    quantity INTEGER NOT NULL DEFAULT 1 CHECK (quantity > 0),
    previous_reward_balance INTEGER NOT NULL CHECK (previous_reward_balance >= 0),
    new_reward_balance INTEGER NOT NULL CHECK (new_reward_balance >= 0),
    previous_stamp_progress INTEGER NOT NULL CHECK (previous_stamp_progress >= 0),
    new_stamp_progress INTEGER NOT NULL CHECK (new_stamp_progress >= 0),
    creation_date TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    idempotency_key TEXT NOT NULL,
    CONSTRAINT member_reward_transactions_merchant_idempotency_key
        UNIQUE (merchant_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_member_reward_transactions_member_created
    ON member_reward_transactions(member_id, creation_date DESC);

CREATE INDEX IF NOT EXISTS idx_member_reward_transactions_merchant_created
    ON member_reward_transactions(merchant_id, creation_date DESC);
