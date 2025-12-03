package model

import (
	"time"
)

// ---- Unified activity feed headers (via SQL UNION, optimized for SQLite) ----

// ActivityHead represents a normalized, cross-entity activity item.
// It unifies companies, invoices, and notes into a single chronological feed.
type ActivityHead struct {
	ItemType   string      `gorm:"column:item_type"` // "company" | "invoice" | "note"
	ItemID     uint        `gorm:"column:item_id"`
	CreatedAt  time.Time   `gorm:"column:created_at"`
	CompanyID  *uint       `gorm:"column:company_id"`  // Only for invoices
	ParentType *ParentType `gorm:"column:parent_type"` // Only for notes
	ParentID   *uint       `gorm:"column:parent_id"`   // Only for notes
}

// GetActivityHeads returns the most recent items across all major entity types
// (companies, invoices, notes) for a given owner/user, ordered by creation time descending.
//
// Internally this uses a SQL UNION to merge multiple tables into a unified feed.
// This avoids complex ORM joins and is efficient for SQLite (and other simple dialects).
//
// Parameters:
//   - userID: owner/tenant identifier (scopes the query)
//   - limit:  max number of feed items to return (defaults to 20 if <= 0)
func (s *Store) GetActivityHeads(userID any, limit int) ([]ActivityHead, error) {
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
LIMIT ?;`

	if err := s.db.Raw(raw, userID, userID, userID, limit).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ---- Batch loaders (prevent N+1 query patterns) ----
//
// Each of the following methods loads all entities of a given type for a
// specific owner, given a slice of IDs. This allows hydrating multiple items
// efficiently when building composite views or feeds.

func (s *Store) CompaniesByIDs(ownerID any, ids []uint) (map[uint]Company, error) {
	out := make(map[uint]Company)
	if len(ids) == 0 {
		return out, nil
	}
	var items []Company
	if err := s.db.
		Where("owner_id = ? AND id IN ?", ownerID, ids).
		Find(&items).Error; err != nil {
		return nil, err
	}
	for _, it := range items {
		out[it.ID] = it
	}
	return out, nil
}

func (s *Store) PeopleByIDs(ownerID any, ids []uint) (map[uint]Person, error) {
	out := make(map[uint]Person)
	if len(ids) == 0 {
		return out, nil
	}
	var items []Person
	if err := s.db.
		Where("owner_id = ? AND id IN ?", ownerID, ids).
		Find(&items).Error; err != nil {
		return nil, err
	}
	for _, it := range items {
		out[it.ID] = it
	}
	return out, nil
}

func (s *Store) InvoicesByIDs(ownerID any, ids []uint) (map[uint]Invoice, error) {
	out := make(map[uint]Invoice)
	if len(ids) == 0 {
		return out, nil
	}
	var items []Invoice
	if err := s.db.
		Where("owner_id = ? AND id IN ?", ownerID, ids).
		Find(&items).Error; err != nil {
		return nil, err
	}
	for _, it := range items {
		out[it.ID] = it
	}
	return out, nil
}

func (s *Store) NotesByIDs(ownerID any, ids []uint) (map[uint]Note, error) {
	out := make(map[uint]Note)
	if len(ids) == 0 {
		return out, nil
	}
	var items []Note
	if err := s.db.
		Where("owner_id = ? AND id IN ?", ownerID, ids).
		Find(&items).Error; err != nil {
		return nil, err
	}
	for _, it := range items {
		out[it.ID] = it
	}
	return out, nil
}

// ---- Convenience: hydrate all related data in one pass ----

// ActivityHydration aggregates feed headers with preloaded entity data.
// This allows building a unified, fully-hydrated activity stream without N+1 queries.
type ActivityHydration struct {
	Heads     []ActivityHead
	Companies map[uint]Company
	People    map[uint]Person
	Invoices  map[uint]Invoice
	Notes     map[uint]Note
}

// LoadActivity loads the latest unified feed (activity heads) and preloads
// all referenced entities (companies, people, invoices, notes) in batch.
//
// This method effectively produces a complete in-memory view of recent activity
// without issuing a separate SQL query per item.
func (s *Store) LoadActivity(ownerID any, limit int) (*ActivityHydration, error) {
	heads, err := s.GetActivityHeads(ownerID, limit)
	if err != nil {
		return nil, err
	}

	// Collect referenced IDs by entity type
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
				case ParentTypeCompany:
					companySet[*h.ParentID] = struct{}{}
				case ParentTypePerson:
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

	// Batch load all entity types in parallel-friendly sequence
	cmap, err := s.CompaniesByIDs(ownerID, toSlice(companySet))
	if err != nil {
		return nil, err
	}
	pmap, err := s.PeopleByIDs(ownerID, toSlice(personSet))
	if err != nil {
		return nil, err
	}
	imap, err := s.InvoicesByIDs(ownerID, toSlice(invoiceSet))
	if err != nil {
		return nil, err
	}
	nmap, err := s.NotesByIDs(ownerID, toSlice(noteSet))
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
