package controller

import (
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

// ---- DTOs für Invoices ----
type APIInvoice struct {
	ID         uint      `json:"id" xml:"id"`
	Number     string    `json:"number" xml:"number"`
	Status     string    `json:"status" xml:"status"`
	Currency   string    `json:"currency" xml:"currency"`
	NetTotal   string    `json:"net_total" xml:"net_total"`
	GrossTotal string    `json:"gross_total" xml:"gross_total"`
	Date       time.Time `json:"date" xml:"date"`
	DueDate    time.Time `json:"due_date" xml:"due_date"`
	CompanyID  uint      `json:"company_id" xml:"company_id"`
}

type APIInvoiceList struct {
	XMLName    struct{}     `json:"-" xml:"invoices"`
	Items      []APIInvoice `json:"items" xml:"invoice"`
	NextCursor string       `json:"next_cursor,omitempty" xml:"next_cursor,omitempty"`
}
