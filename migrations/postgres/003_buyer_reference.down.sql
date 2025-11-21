-- Remove buyer_reference from invoices
ALTER TABLE public.invoices
    DROP COLUMN IF EXISTS buyer_reference;
