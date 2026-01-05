package controller

import (
	"encoding/xml"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/billingcat/crm/model"
	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
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

type APICustomerList struct {
	XMLName struct{}      `json:"-" xml:"customers"`
	Items   []APICustomer `json:"items" xml:"customer"`
	Total   int64         `json:"total" xml:"total,attr"`
	Limit   int           `json:"limit" xml:"limit,attr"`
	Offset  int           `json:"offset" xml:"offset,attr"`
}

type customerListQuery struct {
	Query  string   `query:"q"`
	Tags   []string `query:"tags"`
	Limit  int      `query:"limit"`
	Offset int      `query:"offset"`
}

// apiCustomerList handles GET /api/v1/customers
func (ctrl *controller) apiCustomerList(c echo.Context) error {
	ownerID := apiOwnerID(c)

	var q customerListQuery
	if err := c.Bind(&q); err != nil {
		return respond(c, http.StatusBadRequest, apiError("bad_query", "invalid query params"))
	}

	if q.Limit <= 0 {
		q.Limit = 25
	}
	if q.Limit > 200 {
		q.Limit = 200
	}

	result, err := ctrl.model.SearchCompaniesByTags(ownerID, model.CompanyListFilters{
		Query:  q.Query,
		Tags:   q.Tags,
		Limit:  q.Limit,
		Offset: q.Offset,
	})
	if err != nil {
		return respond(c, http.StatusInternalServerError, apiError("db_error", "could not load customers"))
	}

	items := make([]APICustomer, len(result.Companies))
	for i, comp := range result.Companies {
		items[i] = companyToAPICustomer(&comp)
	}

	return respond(c, http.StatusOK, APICustomerList{
		Items:  items,
		Total:  result.Total,
		Limit:  q.Limit,
		Offset: q.Offset,
	})
}

// apiCustomerGet handles GET /api/v1/customers/:id
func (ctrl *controller) apiCustomerGet(c echo.Context) error {
	ownerID := apiOwnerID(c)

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return respond(c, http.StatusBadRequest, apiError("bad_request", "invalid id"))
	}

	comp, err := ctrl.model.LoadCompany(uint(id), ownerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return respond(c, http.StatusNotFound, apiError("not_found", "customer not found"))
		}
		return respond(c, http.StatusInternalServerError, apiError("db_error", "could not load customer"))
	}

	out := companyToAPICustomer(comp)

	// Add ETag for caching
	c.Response().Header().Set("ETag",
		`W/"cust-`+strconv.FormatUint(uint64(comp.ID), 10)+
			`-`+strconv.FormatInt(comp.UpdatedAt.Unix(), 10)+`"`)

	return respond(c, http.StatusOK, out)
}

// APICustomerCreate is the input for POST /api/v1/customers
type APICustomerCreate struct {
	Name                   string `json:"name" xml:"name"`
	CustomerNumber         string `json:"customer_number,omitempty" xml:"customer_number,omitempty"`
	Address1               string `json:"address1,omitempty" xml:"address1,omitempty"`
	Address2               string `json:"address2,omitempty" xml:"address2,omitempty"`
	Zip                    string `json:"zip,omitempty" xml:"zip,omitempty"`
	City                   string `json:"city,omitempty" xml:"city,omitempty"`
	Country                string `json:"country,omitempty" xml:"country,omitempty"`
	InvoiceEmail           string `json:"invoice_email,omitempty" xml:"invoice_email,omitempty"`
	ContactInvoice         string `json:"contact_invoice,omitempty" xml:"contact_invoice,omitempty"`
	SupplierNumber         string `json:"supplier_number,omitempty" xml:"supplier_number,omitempty"`
	VATID                  string `json:"vat_id,omitempty" xml:"vat_id,omitempty"`
	Background             string `json:"background,omitempty" xml:"background,omitempty"`
	DefaultTaxRate         string `json:"default_tax_rate,omitempty" xml:"default_tax_rate,omitempty"`
	InvoiceCurrency        string `json:"invoice_currency,omitempty" xml:"invoice_currency,omitempty"`
	InvoiceTaxType         string `json:"invoice_tax_type,omitempty" xml:"invoice_tax_type,omitempty"`
	InvoiceOpening         string `json:"invoice_opening,omitempty" xml:"invoice_opening,omitempty"`
	InvoiceFooter          string `json:"invoice_footer,omitempty" xml:"invoice_footer,omitempty"`
	InvoiceExemptionReason string `json:"invoice_exemption_reason,omitempty" xml:"invoice_exemption_reason,omitempty"`
	Tags                   []string `json:"tags,omitempty" xml:"tags>tag,omitempty"`
}

// apiCustomerCreate handles POST /api/v1/customers
func (ctrl *controller) apiCustomerCreate(c echo.Context) error {
	ownerID := apiOwnerID(c)

	var input APICustomerCreate
	if err := c.Bind(&input); err != nil {
		return respond(c, http.StatusBadRequest, apiError("bad_request", "invalid request body"))
	}

	// Validate required fields
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return respond(c, http.StatusBadRequest, apiError("validation_error", "name is required"))
	}

	// Parse default tax rate
	var taxRate decimal.Decimal
	if input.DefaultTaxRate != "" {
		var err error
		taxRate, err = decimal.NewFromString(input.DefaultTaxRate)
		if err != nil {
			return respond(c, http.StatusBadRequest, apiError("validation_error", "invalid default_tax_rate"))
		}
	}

	comp := &model.Company{
		OwnerID:                ownerID,
		Name:                   name,
		CustomerNumber:         strings.TrimSpace(input.CustomerNumber),
		Address1:               strings.TrimSpace(input.Address1),
		Address2:               strings.TrimSpace(input.Address2),
		Zip:                    strings.TrimSpace(input.Zip),
		City:                   strings.TrimSpace(input.City),
		Country:                strings.TrimSpace(input.Country),
		InvoiceEmail:           strings.TrimSpace(input.InvoiceEmail),
		ContactInvoice:         strings.TrimSpace(input.ContactInvoice),
		SupplierNumber:         strings.TrimSpace(input.SupplierNumber),
		VATID:                  strings.TrimSpace(input.VATID),
		Background:             strings.TrimSpace(input.Background),
		DefaultTaxRate:         taxRate,
		InvoiceCurrency:        strings.TrimSpace(input.InvoiceCurrency),
		InvoiceTaxType:         strings.TrimSpace(input.InvoiceTaxType),
		InvoiceOpening:         strings.TrimSpace(input.InvoiceOpening),
		InvoiceFooter:          strings.TrimSpace(input.InvoiceFooter),
		InvoiceExemptionReason: strings.TrimSpace(input.InvoiceExemptionReason),
	}

	if err := ctrl.model.SaveCompany(comp, ownerID, input.Tags); err != nil {
		return respond(c, http.StatusInternalServerError, apiError("db_error", "could not create customer"))
	}

	out := companyToAPICustomer(comp)

	c.Response().Header().Set("Location", "/api/v1/customers/"+strconv.FormatUint(uint64(comp.ID), 10))
	return respond(c, http.StatusCreated, out)
}

// companyToAPICustomer converts a model.Company to APICustomer
func companyToAPICustomer(comp *model.Company) APICustomer {
	contactInfos := make([]APIContactInfo, len(comp.ContactInfos))
	for i, ci := range comp.ContactInfos {
		contactInfos[i] = APIContactInfo{
			ID:        ci.ID,
			CreatedAt: ci.CreatedAt,
			UpdatedAt: ci.UpdatedAt,
			Type:      ci.Type,
			Label:     ci.Label,
			Value:     ci.Value,
		}
	}

	return APICustomer{
		ID:                     comp.ID,
		Name:                   comp.Name,
		CustomerNumber:         comp.CustomerNumber,
		Address1:               comp.Address1,
		Address2:               comp.Address2,
		Zip:                    comp.Zip,
		City:                   comp.City,
		Country:                comp.Country,
		InvoiceEmail:           comp.InvoiceEmail,
		ContactInvoice:         comp.ContactInvoice,
		SupplierNumber:         comp.SupplierNumber,
		VATID:                  comp.VATID,
		Background:             comp.Background,
		ContactInfo:            contactInfos,
		DefaultTaxRate:         comp.DefaultTaxRate.String(),
		InvoiceCurrency:        comp.InvoiceCurrency,
		InvoiceTaxType:         comp.InvoiceTaxType,
		InvoiceOpening:         comp.InvoiceOpening,
		InvoiceFooter:          comp.InvoiceFooter,
		InvoiceExemptionReason: comp.InvoiceExemptionReason,
		CreatedAt:              comp.CreatedAt,
		UpdatedAt:              comp.UpdatedAt,
	}
}
