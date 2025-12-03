// model/invoice_service.go
package model

import (
	"errors"
	"strconv"

	"gorm.io/gorm"
)

// InvoiceListQuery captures filter, paging, and sorting options for listing invoices.
type InvoiceListQuery struct {
	Status    string // Optional: filter by status (application-defined values, e.g., "open", "paid")
	CompanyID uint   // Optional: restrict to a single company
	Limit     int    // Page size (1–200); defaults to 50 when out of range
	Cursor    string // Simple offset cursor encoded as a string: "0", "50", ...
	Sort      string // Sort mode: "date_desc" (default), "date_asc", "created_desc"
}

// ListInvoices returns a page of invoices for the given owner along with the next cursor.
// Owner-scoped and safe to call repeatedly for pagination.
//
// Paging model:
//   - Uses an offset-based cursor encoded as a string (q.Cursor).
//   - Fetches Limit+1 rows to determine if there is a next page; if so, trims to Limit and
//     returns nextCursor = offset + Limit (as string).
//
// Filters:
//   - Status (exact match)
//   - CompanyID
//
// Sorting:
//   - "date_desc" (default): ORDER BY date DESC
//   - "date_asc":            ORDER BY date ASC
//   - "created_desc":        ORDER BY created_at DESC
func (s *Store) ListInvoices(ownerID uint, q InvoiceListQuery) (items []Invoice, nextCursor string, err error) {
	// Clamp/normalize limit
	if q.Limit <= 0 || q.Limit > 200 {
		q.Limit = 50
	}

	// Decode offset cursor
	offset := 0
	if q.Cursor != "" {
		if n, e := strconv.Atoi(q.Cursor); e == nil && n >= 0 {
			offset = n
		}
	}

	// Base query: owner scope
	db := s.db.Model(&Invoice{}).Where("owner_id = ?", ownerID)

	// Optional filters
	if q.Status != "" {
		db = db.Where("status = ?", q.Status)
	}
	if q.CompanyID != 0 {
		db = db.Where("company_id = ?", q.CompanyID)
	}

	// Sorting
	switch q.Sort {
	case "date_asc":
		db = db.Order("date asc")
	case "created_desc":
		db = db.Order("created_at desc")
	default:
		db = db.Order("date desc")
	}

	// Page fetch (limit+1 to compute "has next")
	var invs []Invoice
	if err = db.Offset(offset).Limit(q.Limit + 1).Find(&invs).Error; err != nil {
		return nil, "", err
	}

	// Derive next cursor
	if len(invs) > q.Limit {
		invs = invs[:q.Limit]
		nextCursor = strconv.Itoa(offset + q.Limit)
	}
	return invs, nextCursor, nil
}

// GetInvoiceByOwner loads a single invoice by id, ensuring it belongs to the given owner.
// Returns gorm.ErrRecordNotFound when the invoice does not exist within the owner scope.
func (s *Store) GetInvoiceByOwner(ownerID uint, id uint) (*Invoice, error) {
	var inv Invoice
	if err := s.db.Where("owner_id = ?", ownerID).First(&inv, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &inv, nil
}
