BEGIN;
ALTER TABLE settings
  DROP COLUMN IF EXISTS customer_number_prefix,
  DROP COLUMN IF EXISTS customer_number_width,
  DROP COLUMN IF EXISTS customer_number_counter;
COMMIT;
