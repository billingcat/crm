CREATE TABLE IF NOT EXISTS email_templates (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    owner_id    INTEGER NOT NULL,
    company_id  INTEGER NOT NULL DEFAULT 0,
    kind        TEXT    NOT NULL,
    subject     TEXT    NOT NULL DEFAULT '',
    body        TEXT    NOT NULL DEFAULT ''
);

CREATE UNIQUE INDEX idx_email_templates_unique
    ON email_templates(owner_id, company_id, kind);
