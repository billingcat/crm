BEGIN;

DROP INDEX IF EXISTS idx_user_view;

CREATE UNIQUE INDEX IF NOT EXISTS idx_recent_view
    ON recent_views (user_id, entity_type, entity_id);

COMMIT;
