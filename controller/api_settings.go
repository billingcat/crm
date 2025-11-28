package controller

import "encoding/xml"

type APISettings struct {
	CompanyName           string `xml:"company_name"`
	InvoiceContact        string `xml:"invoice_contact"`
	InvoiceEMail          string `xml:"invoice_email"`
	ZIP                   string `xml:"zip"`
	Address1              string `xml:"address1"`
	Address2              string `xml:"address2"`
	City                  string `xml:"city"`
	CountryCode           string `xml:"country_code"`
	VATID                 string `xml:"vat_id"`
	TAXNumber             string `xml:"tax_number"`
	InvoiceNumberTemplate string `xml:"invoice_number_template"`
	UseLocalCounter       bool   `xml:"use_local_counter"`
	BankIBAN              string `xml:"bank_iban"`
	BankName              string `xml:"bank_name"`
	BankBIC               string `xml:"bank_bic"`
	CustomerNumberPrefix  string `xml:"customer_number_prefix"`
	CustomerNumberWidth   int    `xml:"customer_number_width"`
	CustomerNumberCounter int64  `xml:"customer_number_counter"`
}

type ExportSettings struct {
	XMLName xml.Name    `xml:"settings"`
	Version string      `xml:"version,attr,omitempty"`
	Setting APISettings `xml:"setting"`
}
