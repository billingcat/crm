BEGIN;
ALTER TABLE settings DROP COLUMN customer_number_prefix;
ALTER TABLE settings DROP COLUMN customer_number_width;
ALTER TABLE settings DROP COLUMN customer_number_counter;
COMMIT;

