-- 004_create_abuse_reports.up.sql

CREATE TABLE IF NOT EXISTS abuse_reports (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id     UUID NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    reporter_ip   INET NOT NULL,
    reason        TEXT NOT NULL,
    evidence_url  TEXT,
    status        VARCHAR(12) NOT NULL DEFAULT 'open'
                  CHECK (status IN ('open', 'investigating', 'resolved', 'dismissed')),
    admin_notes   TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at   TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_abuse_reports_domain_id ON abuse_reports(domain_id);
CREATE INDEX IF NOT EXISTS idx_abuse_reports_status ON abuse_reports(status);
