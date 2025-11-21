-- Add buyer_reference to invoices
ALTER TABLE invoices
    ADD COLUMN buyer_reference text;
