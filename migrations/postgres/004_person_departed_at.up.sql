-- Add departed_at column to track when a person left their company
ALTER TABLE people ADD COLUMN departed_at TIMESTAMP;
