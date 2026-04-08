CREATE TABLE IF NOT EXISTS audit_logs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    owner_id    INTEGER NOT NULL,
    user_id     INTEGER NOT NULL,
    action      TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id   INTEGER NOT NULL,
    summary     TEXT
);

CREATE INDEX idx_audit_logs_owner_id ON audit_logs(owner_id);
CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at);
