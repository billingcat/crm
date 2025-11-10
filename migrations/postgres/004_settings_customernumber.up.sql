BEGIN;
ALTER TABLE settings
  ADD COLUMN IF NOT EXISTS customer_number_prefix TEXT,
  ADD COLUMN IF NOT EXISTS customer_number_width  INTEGER,
  ADD COLUMN IF NOT EXISTS customer_number_counter  BIGINT;
COMMIT;
