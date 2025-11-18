-- Revert unique index on recent_views

BEGIN;

DROP INDEX IF EXISTS idx_recent_view;

CREATE INDEX IF NOT EXISTS idx_user_view
    ON recent_views (user_id, entity_type, entity_id);

COMMIT;