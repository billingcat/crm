// controller/api_invoices.go
package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/billingcat/crm/model"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

type invoiceListQuery struct {
	Status    string `query:"status"`
	CompanyID uint   `query:"company_id"`
	Limit     int    `query:"limit"`
	Cursor    string `query:"cursor"`
	Sort      string `query:"sort"`
}

func (ctrl *controller) apiInvoiceList(c echo.Context) error {
	ownerID := apiOwnerID(c)
	var q invoiceListQuery
	if err := c.Bind(&q); err != nil {
		return respond(c, http.StatusBadRequest, apiError("bad_query", "invalid query params"))
	}
	invs, next, err := ctrl.model.ListInvoices(ownerID, model.InvoiceListQuery{
		Status:    q.Status,
		CompanyID: q.CompanyID,
		Limit:     q.Limit,
		Cursor:    q.Cursor,
		Sort:      q.Sort,
	})
	if err != nil {
		return respond(c, http.StatusInternalServerError, apiError("db_error", "could not load invoices"))
	}

	items := make([]APIInvoice, len(invs))
	for i, v := range invs {
		items[i] = APIInvoice{
			ID:             v.ID,
			Number:         v.Number,
			Status:         string(v.Status),
			Currency:       v.Currency,
			NetTotal:       v.NetTotal.String(),
			GrossTotal:     v.GrossTotal.String(),
			Date:           v.Date,
			DueDate:        v.DueDate,
			OccurrenceDate: v.OccurrenceDate,
			CompanyID:      v.CompanyID,
			CreatedAt:      v.CreatedAt,
			UpdatedAt:      v.UpdatedAt,
		}
	}
	return respond(c, http.StatusOK, APIInvoiceList{Items: items, NextCursor: next})
}

func (ctrl *controller) apiInvoiceGet(c echo.Context) error {
	ownerID := apiOwnerID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return respond(c, http.StatusBadRequest, apiError("bad_request", "invalid id"))
	}
	inv, err := ctrl.model.LoadInvoice(uint(id), ownerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return respond(c, http.StatusNotFound, apiError("not_found", "invoice not found"))
		}
		return respond(c, http.StatusInternalServerError, apiError("db_error", "could not load invoice"))
	}

	// Ensure totals and tax amounts are up to date based on positions
	inv.RecomputeTotals()

	positions := make([]APIInvoicePosition, len(inv.InvoicePositions))
	for i, p := range inv.InvoicePositions {
		positions[i] = APIInvoicePosition{
			ID:         p.ID,
			Position:   p.Position,
			UnitCode:   p.UnitCode,
			Text:       p.Text,
			Quantity:   p.Quantity.String(),
			TaxRate:    p.TaxRate.String(),
			NetPrice:   p.NetPrice.String(),
			GrossPrice: p.GrossPrice.String(),
			LineTotal:  p.LineTotal.String(),
		}
	}

	taxAmounts := make([]APITaxAmount, len(inv.TaxAmounts))
	for i, t := range inv.TaxAmounts {
		taxAmounts[i] = APITaxAmount{
			Rate:   t.Rate.String(),
			Amount: t.Amount.String(),
		}
	}

	out := APIInvoice{
		ID:               inv.ID,
		Number:           inv.Number,
		Status:           string(inv.Status),
		Currency:         inv.Currency,
		NetTotal:         inv.NetTotal.String(),
		GrossTotal:       inv.GrossTotal.String(),
		Date:             inv.Date,
		DueDate:          inv.DueDate,
		CompanyID:        inv.CompanyID,
		ContactInvoice:   inv.ContactInvoice,
		Counter:          inv.Counter,
		ExemptionReason:  inv.ExemptionReason,
		Footer:           inv.Footer,
		Opening:          inv.Opening,
		OccurrenceDate:   inv.OccurrenceDate,
		OrderNumber:      inv.OrderNumber,
		BuyerReference:   inv.BuyerReference,
		SupplierNumber:   inv.SupplierNumber,
		TaxNumber:        inv.TaxNumber,
		TaxType:          inv.TaxType,
		TemplateID:       inv.TemplateID,
		IssuedAt:         inv.IssuedAt,
		PaidAt:           inv.PaidAt,
		VoidedAt:         inv.VoidedAt,
		CreatedAt:        inv.CreatedAt,
		UpdatedAt:        inv.UpdatedAt,
		InvoicePositions: positions,
		TaxAmounts:       taxAmounts,
	}
	// optional: ETag for caching
	c.Response().Header().Set("ETag",
		`W/"inv-`+strconv.FormatUint(uint64(inv.ID), 10)+
			`-`+strconv.FormatInt(inv.UpdatedAt.Unix(), 10)+`"`)

	return respond(c, http.StatusOK, out)
}
