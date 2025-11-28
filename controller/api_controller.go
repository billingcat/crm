package controller

import (
	"encoding/xml"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

type APIError struct {
	Code    string `json:"code" xml:"code"`
	Message string `json:"message" xml:"message"`
}

func apiError(code, msg string) *APIError { return &APIError{Code: code, Message: msg} }

func wantsXML(c echo.Context) bool {
	if c.QueryParam("format") == "xml" {
		return true
	}
	accept := c.Request().Header.Get(echo.HeaderAccept)
	return strings.Contains(accept, "application/xml") || strings.Contains(accept, "text/xml")
}
func respond(c echo.Context, status int, v any) error {
	if wantsXML(c) {
		return c.XML(status, v)
	}
	return c.JSON(status, v)
}

// ---- DTOs for invoices ----
type APIInvoice struct {
	ID               uint                 `json:"id" xml:"id,attr"`
	Number           string               `json:"number" xml:"number"`
	Status           string               `json:"status" xml:"status"`
	Currency         string               `json:"currency" xml:"currency"`
	NetTotal         string               `json:"net_total" xml:"net_total"`
	GrossTotal       string               `json:"gross_total" xml:"gross_total"`
	Date             time.Time            `json:"date" xml:"date"`
	DueDate          time.Time            `json:"due_date" xml:"due_date"`
	CompanyID        uint                 `json:"company_id" xml:"company_id"`
	ContactInvoice   string               `json:"contact_invoice,omitempty" xml:"contact_invoice,omitempty"`
	Counter          uint                 `json:"counter,omitempty" xml:"counter,omitempty"`
	ExemptionReason  string               `json:"exemption_reason,omitempty" xml:"exemption_reason,omitempty"`
	Footer           string               `json:"footer,omitempty" xml:"footer,omitempty"`
	Opening          string               `json:"opening,omitempty" xml:"opening,omitempty"`
	OccurrenceDate   time.Time            `json:"occurrence_date,omitempty" xml:"occurrence_date,omitempty"`
	OrderNumber      string               `json:"order_number,omitempty" xml:"order_number,omitempty"`
	BuyerReference   string               `json:"buyer_reference,omitempty" xml:"buyer_reference,omitempty"`
	SupplierNumber   string               `json:"supplier_number,omitempty" xml:"supplier_number,omitempty"`
	TaxNumber        string               `json:"tax_number,omitempty" xml:"tax_number,omitempty"`
	TaxType          string               `json:"tax_type,omitempty" xml:"tax_type,omitempty"`
	TemplateID       *uint                `json:"template_id,omitempty" xml:"template_id,omitempty"`
	IssuedAt         *time.Time           `json:"issued_at,omitempty" xml:"issued_at,omitempty"`
	PaidAt           *time.Time           `json:"paid_at,omitempty" xml:"paid_at,omitempty"`
	VoidedAt         *time.Time           `json:"voided_at,omitempty" xml:"voided_at,omitempty"`
	CreatedAt        time.Time            `json:"created_at" xml:"created_at"`
	UpdatedAt        time.Time            `json:"updated_at" xml:"updated_at"`
	InvoicePositions []APIInvoicePosition `json:"invoice_positions,omitempty" xml:"invoice_positions>position,omitempty"`
	TaxAmounts       []APITaxAmount       `json:"tax_amounts,omitempty" xml:"tax_amounts>tax_amount,omitempty"`
}

type APIInvoicePosition struct {
	ID         uint   `json:"id" xml:"id"`
	Position   int    `json:"position" xml:"position"`
	UnitCode   string `json:"unit_code" xml:"unit_code"`
	Text       string `json:"text" xml:"text"`
	Quantity   string `json:"quantity" xml:"quantity"`
	TaxRate    string `json:"tax_rate" xml:"tax_rate"`
	NetPrice   string `json:"net_price" xml:"net_price"`
	GrossPrice string `json:"gross_price" xml:"gross_price"`
	LineTotal  string `json:"line_total" xml:"line_total"`
}

type APITaxAmount struct {
	Rate   string `json:"rate" xml:"rate"`
	Amount string `json:"amount" xml:"amount"`
}

type APIInvoiceList struct {
	XMLName    struct{}     `json:"-" xml:"invoices"`
	Items      []APIInvoice `json:"items" xml:"invoice"`
	NextCursor string       `json:"next_cursor,omitempty" xml:"next_cursor,omitempty"`
}

type ExportInvoices struct {
	XMLName  xml.Name     `xml:"invoices"`
	Version  string       `xml:"version,attr,omitempty"`
	Invoices []APIInvoice `xml:"invoice"`
}
