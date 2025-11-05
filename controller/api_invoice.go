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
			ID:         v.ID,
			Number:     v.Number,
			Status:     string(v.Status),
			Currency:   v.Currency,
			NetTotal:   v.NetTotal.String(),
			GrossTotal: v.GrossTotal.String(),
			Date:       v.Date,
			DueDate:    v.DueDate,
			CompanyID:  v.CompanyID,
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
	inv, err := ctrl.model.GetInvoiceByOwner(ownerID, uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return respond(c, http.StatusNotFound, apiError("not_found", "invoice not found"))
		}
		return respond(c, http.StatusInternalServerError, apiError("db_error", "could not load invoice"))
	}

	out := APIInvoice{
		ID:         inv.ID,
		Number:     inv.Number,
		Status:     string(inv.Status),
		Currency:   inv.Currency,
		NetTotal:   inv.NetTotal.String(),
		GrossTotal: inv.GrossTotal.String(),
		Date:       inv.Date,
		DueDate:    inv.DueDate,
		CompanyID:  inv.CompanyID,
	}
	// optional: ETag for caching
	c.Response().Header().Set("ETag",
		`W/"inv-`+strconv.FormatUint(uint64(inv.ID), 10)+
			`-`+strconv.FormatInt(inv.UpdatedAt.Unix(), 10)+`"`)

	return respond(c, http.StatusOK, out)
}
