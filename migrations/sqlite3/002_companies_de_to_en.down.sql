BEGIN TRANSACTION;

-- rollback EN → DE
CREATE TABLE companies_old AS
SELECT
    id,
    created_at,
    updated_at,
    deleted_at,
    background,
    name,
    customer_number   AS kundennummer,
    invoice_email     AS rechnung_email,
    contact_invoice,
    supplier_number,
    address1          AS adresse1,
    address2          AS adresse2,
    country           AS land,
    vat_id,
    invoice_opening,
    invoice_currency,
    invoice_tax_type,
    invoice_footer,
    zip               AS plz,
    city              AS ort,
    invoice_exemption_reason,
    owner_id,
    default_tax_rate
FROM companies;

DROP TABLE companies;
ALTER TABLE companies_old RENAME TO companies;

COMMIT;