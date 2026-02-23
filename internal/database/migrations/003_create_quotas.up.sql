-- 003_create_quotas.up.sql

CREATE TABLE IF NOT EXISTS quotas (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID UNIQUE NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    base_limit    INT NOT NULL DEFAULT 2,
    bonus_limit   INT NOT NULL DEFAULT 0,
    used_slots    INT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_used_slots CHECK (used_slots >= 0),
    CONSTRAINT chk_limits CHECK (used_slots <= base_limit + bonus_limit)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_quotas_user_id ON quotas(user_id);
