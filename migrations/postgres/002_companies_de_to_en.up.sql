BEGIN;

-- companies: DE → EN
ALTER TABLE public.companies RENAME COLUMN adresse1       TO address1;
ALTER TABLE public.companies RENAME COLUMN adresse2       TO address2;
ALTER TABLE public.companies RENAME COLUMN kundennummer   TO customer_number;
ALTER TABLE public.companies RENAME COLUMN rechnung_email TO invoice_email;
ALTER TABLE public.companies RENAME COLUMN land           TO country;
ALTER TABLE public.companies RENAME COLUMN plz            TO zip;
ALTER TABLE public.companies RENAME COLUMN ort            TO city;

COMMIT;
