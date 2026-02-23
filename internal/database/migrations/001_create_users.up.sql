-- 001_create_users.up.sql

CREATE TABLE IF NOT EXISTS users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    github_id     BIGINT UNIQUE NOT NULL,
    username      VARCHAR(39) NOT NULL,
    email         VARCHAR(255),
    avatar_url    TEXT,
    type          VARCHAR(2) NOT NULL             -- 'PF' ou 'PJ'
                  CHECK (type IN ('PF', 'PJ')),
    doc_hash      VARCHAR(64) NOT NULL,           -- SHA-256 do documento
    doc_salt      VARCHAR(32) NOT NULL,           -- Salt único por usuário
    role          VARCHAR(10) NOT NULL DEFAULT 'user'
                  CHECK (role IN ('user', 'admin', 'system')),
    verified      BOOLEAN NOT NULL DEFAULT FALSE,
    bio           TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_github_id ON users(github_id);
CREATE INDEX IF NOT EXISTS idx_users_doc_hash ON users(doc_hash);
CREATE INDEX IF NOT EXISTS idx_users_type ON users(type);
