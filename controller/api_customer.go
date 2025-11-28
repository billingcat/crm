package controller

import (
	"encoding/xml"
	"time"
)

type ExportCustomers struct {
	XMLName   xml.Name      `xml:"customers"`
	Version   string        `xml:"version,attr,omitempty"`
	Customers []APICustomer `xml:"customer"`
}

type APICustomer struct {
	ID                     uint             `json:"id" xml:"id,attr"`
	Name                   string           `json:"name" xml:"name"`
	CustomerNumber         string           `json:"customer_number,omitempty" xml:"customer_number,omitempty"`
	Address1               string           `json:"address1,omitempty" xml:"address1,omitempty"`
	Address2               string           `json:"address2,omitempty" xml:"address2,omitempty"`
	Zip                    string           `json:"zip,omitempty" xml:"zip,omitempty"`
	City                   string           `json:"city,omitempty" xml:"city,omitempty"`
	Country                string           `json:"country,omitempty" xml:"country,omitempty"`
	InvoiceEmail           string           `json:"invoice_email,omitempty" xml:"invoice_email,omitempty"`
	ContactInvoice         string           `json:"contact_invoice,omitempty" xml:"contact_invoice,omitempty"`
	SupplierNumber         string           `json:"supplier_number,omitempty" xml:"supplier_number,omitempty"`
	VATID                  string           `json:"vat_id,omitempty" xml:"vat_id,omitempty"`
	Background             string           `json:"background,omitempty" xml:"background,omitempty"`
	Notes                  []APINote        `json:"notes,omitempty" xml:"notes>note,omitempty"`
	ContactInfo            []APIContactInfo `json:"contact_infos,omitempty" xml:"contact_infos>contact_info,omitempty"`
	DefaultTaxRate         string           `json:"default_tax_rate,omitempty" xml:"default_tax_rate,omitempty"`
	InvoiceCurrency        string           `json:"invoice_currency,omitempty" xml:"invoice_currency,omitempty"`
	InvoiceTaxType         string           `json:"invoice_tax_type,omitempty" xml:"invoice_tax_type,omitempty"`
	InvoiceOpening         string           `json:"invoice_opening,omitempty" xml:"invoice_opening,omitempty"`
	InvoiceFooter          string           `json:"invoice_footer,omitempty" xml:"invoice_footer,omitempty"`
	InvoiceExemptionReason string           `json:"invoice_exemption_reason,omitempty" xml:"invoice_exemption_reason,omitempty"`

	CreatedAt time.Time `json:"created_at" xml:"created_at"`
	UpdatedAt time.Time `json:"updated_at" xml:"updated_at"`
}
