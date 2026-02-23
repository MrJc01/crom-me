-- 002_create_domains.up.sql

CREATE TABLE IF NOT EXISTS domains (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subdomain     VARCHAR(63) UNIQUE NOT NULL,
    target        VARCHAR(255) NOT NULL,
    record_type   VARCHAR(5) NOT NULL DEFAULT 'CNAME'
                  CHECK (record_type IN ('A', 'AAAA', 'CNAME', 'TXT')),
    dns_record_id VARCHAR(32),
    status        VARCHAR(10) NOT NULL DEFAULT 'pending'
                  CHECK (status IN ('pending', 'active', 'suspended', 'rejected')),
    purpose       TEXT NOT NULL,
    project_name  VARCHAR(100) NOT NULL DEFAULT 'Não Informado',
    contact_email VARCHAR(255) NOT NULL DEFAULT 'N/A',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at    TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_domains_subdomain ON domains(subdomain);
CREATE INDEX IF NOT EXISTS idx_domains_user_id ON domains(user_id);
CREATE INDEX IF NOT EXISTS idx_domains_status ON domains(status);
