BEGIN TRANSACTION;

-- companies: DE → EN
CREATE TABLE companies_new AS
SELECT
    id,
    created_at,
    updated_at,
    deleted_at,
    background,
    name,
    kundennummer     AS customer_number,
    rechnung_email   AS invoice_email,
    contact_invoice,
    supplier_number,
    adresse1         AS address1,
    adresse2         AS address2,
    land             AS country,
    vat_id,
    invoice_opening,
    invoice_currency,
    invoice_tax_type,
    invoice_footer,
    plz              AS zip,
    ort              AS city,
    invoice_exemption_reason,
    owner_id,
    default_tax_rate
FROM companies;

DROP TABLE companies;
ALTER TABLE companies_new RENAME TO companies;

COMMIT;
