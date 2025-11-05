BEGIN;

-- rollback EN → DE
ALTER TABLE public.companies RENAME COLUMN city            TO ort;
ALTER TABLE public.companies RENAME COLUMN zip             TO plz;
ALTER TABLE public.companies RENAME COLUMN country         TO land;
ALTER TABLE public.companies RENAME COLUMN invoice_email   TO rechnung_email;
ALTER TABLE public.companies RENAME COLUMN customer_number TO kundennummer;
ALTER TABLE public.companies RENAME COLUMN address2        TO adresse2;
ALTER TABLE public.companies RENAME COLUMN address1        TO adresse1;

COMMIT;

