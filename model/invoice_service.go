// model/invoice_service.go
package model

import (
	"errors"
	"strconv"

	"gorm.io/gorm"
)

type InvoiceListQuery struct {
	Status    string
	CompanyID uint
	Limit     int
	Cursor    string // einfacher Offset-Cursor: "0", "50", ...
	Sort      string // "date_desc" (default), "date_asc", "created_desc"
}

func (crmdb *CRMDatenbank) ListInvoices(ownerID uint, q InvoiceListQuery) (items []Invoice, nextCursor string, err error) {
	if q.Limit <= 0 || q.Limit > 200 {
		q.Limit = 50
	}
	offset := 0
	if q.Cursor != "" {
		if n, e := strconv.Atoi(q.Cursor); e == nil && n >= 0 {
			offset = n
		}
	}

	db := crmdb.db.Model(&Invoice{}).Where("owner_id = ?", ownerID)

	if q.Status != "" {
		db = db.Where("status = ?", q.Status)
	}
	if q.CompanyID != 0 {
		db = db.Where("company_id = ?", q.CompanyID)
	}

	switch q.Sort {
	case "date_asc":
		db = db.Order("date asc")
	case "created_desc":
		db = db.Order("created_at desc")
	default:
		db = db.Order("date desc")
	}

	var invs []Invoice
	if err = db.Offset(offset).Limit(q.Limit + 1).Find(&invs).Error; err != nil {
		return nil, "", err
	}

	if len(invs) > q.Limit {
		invs = invs[:q.Limit]
		nextCursor = strconv.Itoa(offset + q.Limit)
	}
	return invs, nextCursor, nil
}

func (crmdb *CRMDatenbank) GetInvoiceByOwner(ownerID uint, id uint) (*Invoice, error) {
	var inv Invoice
	if err := crmdb.db.Where("owner_id = ?", ownerID).First(&inv, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &inv, nil
}
