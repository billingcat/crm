BEGIN;
ALTER TABLE settings ADD COLUMN customer_number_prefix TEXT;
ALTER TABLE settings ADD COLUMN customer_number_width  INTEGER;
ALTER TABLE settings ADD COLUMN customer_number_counter  INTEGER;
COMMIT;
