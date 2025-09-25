package model

import (
	"time"
)

// ---- Unified Feed Kopfzeilen (UNION über SQLite) ----

type ActivityHead struct {
	ItemType   string    `gorm:"column:item_type"` // "company" | "invoice" | "note"
	ItemID     uint      `gorm:"column:item_id"`
	CreatedAt  time.Time `gorm:"column:created_at"`
	CompanyID  *uint     `gorm:"column:company_id"`  // nur für invoices
	ParentType *string   `gorm:"column:parent_type"` // nur für notes
	ParentID   *uint     `gorm:"column:parent_id"`   // nur für notes
}

// Global neueste Items über alle Typen.
func (crmdb *CRMDatenbank) GetActivityHeads(ownerID any, limit int) ([]ActivityHead, error) {
	if limit <= 0 {
		limit = 20
	}
	var rows []ActivityHead

	raw := `
SELECT CAST('company' AS text) AS item_type,
       CAST(id AS bigint)      AS item_id,
       created_at,
       CAST(NULL AS bigint)    AS company_id,
       CAST(NULL AS text)      AS parent_type,
       CAST(NULL AS bigint)    AS parent_id
FROM companies
WHERE owner_id = ?

UNION ALL

SELECT CAST('invoice' AS text) AS item_type,
       CAST(id AS bigint)      AS item_id,
       created_at,
       CAST(company_id AS bigint),
       CAST(NULL AS text)      AS parent_type,
       CAST(NULL AS bigint)    AS parent_id
FROM invoices
WHERE owner_id = ?

UNION ALL

SELECT CAST('note' AS text)    AS item_type,
       CAST(id AS bigint)      AS item_id,
       created_at,
       CAST(NULL AS bigint)    AS company_id,
       CAST(parent_type AS text),
       CAST(parent_id AS bigint)
FROM notes
WHERE owner_id = ?

ORDER BY created_at DESC
LIMIT ?;	`
	if err := crmdb.db.Raw(raw, ownerID, ownerID, ownerID, limit).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ---- Batch-Loader (verhindern N+1) ----

func (crmdb *CRMDatenbank) CompaniesByIDs(ownerID any, ids []uint) (map[uint]Company, error) {
	out := make(map[uint]Company)
	if len(ids) == 0 {
		return out, nil
	}
	var items []Company
	if err := crmdb.db.
		Where("owner_id = ? AND id IN ?", ownerID, ids).
		Find(&items).Error; err != nil {
		return nil, err
	}
	for _, it := range items {
		out[it.ID] = it
	}
	return out, nil
}

func (crmdb *CRMDatenbank) PeopleByIDs(ownerID any, ids []uint) (map[uint]Person, error) {
	out := make(map[uint]Person)
	if len(ids) == 0 {
		return out, nil
	}
	var items []Person
	if err := crmdb.db.
		Where("owner_id = ? AND id IN ?", ownerID, ids).
		Find(&items).Error; err != nil {
		return nil, err
	}
	for _, it := range items {
		out[it.ID] = it
	}
	return out, nil
}

func (crmdb *CRMDatenbank) InvoicesByIDs(ownerID any, ids []uint) (map[uint]Invoice, error) {
	out := make(map[uint]Invoice)
	if len(ids) == 0 {
		return out, nil
	}
	var items []Invoice
	if err := crmdb.db.
		Where("owner_id = ? AND id IN ?", ownerID, ids).
		Find(&items).Error; err != nil {
		return nil, err
	}
	for _, it := range items {
		out[it.ID] = it
	}
	return out, nil
}

func (crmdb *CRMDatenbank) NotesByIDs(ownerID any, ids []uint) (map[uint]Note, error) {
	out := make(map[uint]Note)
	if len(ids) == 0 {
		return out, nil
	}
	var items []Note
	if err := crmdb.db.
		Where("owner_id = ? AND id IN ?", ownerID, ids).
		Find(&items).Error; err != nil {
		return nil, err
	}
	for _, it := range items {
		out[it.ID] = it
	}
	return out, nil
}

// ---- Komfort: alles in einem Rutsch laden ----

type ActivityHydration struct {
	Heads     []ActivityHead
	Companies map[uint]Company
	People    map[uint]Person
	Invoices  map[uint]Invoice
	Notes     map[uint]Note
}

func (crmdb *CRMDatenbank) LoadActivity(ownerID any, limit int) (*ActivityHydration, error) {
	heads, err := crmdb.GetActivityHeads(ownerID, limit)
	if err != nil {
		return nil, err
	}

	// IDs einsammeln
	companySet := make(map[uint]struct{})
	personSet := make(map[uint]struct{})
	invoiceSet := make(map[uint]struct{})
	noteSet := make(map[uint]struct{})

	for _, h := range heads {
		switch h.ItemType {
		case "company":
			companySet[h.ItemID] = struct{}{}
		case "invoice":
			invoiceSet[h.ItemID] = struct{}{}
			if h.CompanyID != nil {
				companySet[*h.CompanyID] = struct{}{}
			}
		case "note":
			noteSet[h.ItemID] = struct{}{}
			if h.ParentType != nil && h.ParentID != nil {
				switch *h.ParentType {
				case "companies":
					companySet[*h.ParentID] = struct{}{}
				case "people":
					personSet[*h.ParentID] = struct{}{}
				}
			}
		}
	}

	toSlice := func(m map[uint]struct{}) []uint {
		out := make([]uint, 0, len(m))
		for id := range m {
			out = append(out, id)
		}
		return out
	}

	// Batch-Ladevorgänge
	cmap, err := crmdb.CompaniesByIDs(ownerID, toSlice(companySet))
	if err != nil {
		return nil, err
	}
	pmap, err := crmdb.PeopleByIDs(ownerID, toSlice(personSet))
	if err != nil {
		return nil, err
	}
	imap, err := crmdb.InvoicesByIDs(ownerID, toSlice(invoiceSet))
	if err != nil {
		return nil, err
	}
	nmap, err := crmdb.NotesByIDs(ownerID, toSlice(noteSet))
	if err != nil {
		return nil, err
	}

	return &ActivityHydration{
		Heads:     heads,
		Companies: cmap,
		People:    pmap,
		Invoices:  imap,
		Notes:     nmap,
	}, nil
}
