-- Add buyer_reference to invoices
ALTER TABLE public.invoices
    ADD COLUMN buyer_reference text;
