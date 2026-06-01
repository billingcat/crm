CREATE TABLE IF NOT EXISTS email_templates (
    id          BIGSERIAL PRIMARY KEY,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    owner_id    BIGINT NOT NULL,
    company_id  BIGINT NOT NULL DEFAULT 0,
    kind        TEXT   NOT NULL,
    subject     TEXT   NOT NULL DEFAULT '',
    body        TEXT   NOT NULL DEFAULT ''
);

CREATE UNIQUE INDEX idx_email_templates_unique
    ON email_templates(owner_id, company_id, kind);
