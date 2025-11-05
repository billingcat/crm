package model

import (
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Company is a legal entity (organization).
// It is owner-scoped (OwnerID) and may have invoices, contact infos, notes,
// and people (contacts) associated with it.
type Company struct {
	gorm.Model
	Address1               string          `gorm:"column:address1"`
	Address2               string          `gorm:"column:address2"`
	Background             string          `gorm:"column:background"` // Free-form internal notes about the company
	ContactInvoice         string          `gorm:"column:contact_invoice"`
	DefaultTaxRate         decimal.Decimal `gorm:"column:default_tax_rate;type:decimal(20,8);"` // Monetary precision
	InvoiceCurrency        string          `gorm:"column:invoice_currency"`
	InvoiceExemptionReason string          `gorm:"column:invoice_exemption_reason"`
	InvoiceFooter          string          `gorm:"column:invoice_footer"`
	InvoiceOpening         string          `gorm:"column:invoice_opening"`
	Invoices               []Invoice       `gorm:"foreignKey:CompanyID"`
	InvoiceTaxType         string          `gorm:"column:invoice_tax_type"`
	CustomerNumber         string          `gorm:"column:customer_number"`
	Country                string          `gorm:"column:country"`
	Name                   string          `gorm:"column:name"`
	City                   string          `gorm:"column:city"`
	OwnerID                uint            `gorm:"column:owner_id"` // Tenant/account scope
	ContactInfos           []ContactInfo   `gorm:"polymorphic:Parent;polymorphicValue:companies"`
	Contacts               []*Person       `gorm:"-"` // Computed/loaded separately; ignored by GORM
	Zip                    string          `gorm:"column:zip"`
	InvoiceEmail           string          `gorm:"column:invoice_email"`
	SupplierNumber         string          `gorm:"column:supplier_number"`
	VATID                  string          `gorm:"column:vat_id"` // VAT identification number
	Notes                  []Note          `gorm:"polymorphic:Parent;constraint:OnDelete:CASCADE;"`
}

var ErrNotAllowed = fmt.Errorf("not allowed")

// SaveCompany upserts a company, fully replaces its ContactInfos, and replaces its tags.
// Transactional and owner-scoped.
//
// Semantics:
//   - ContactInfos: "replace" semantics → delete existing rows, then insert the provided set.
//   - Tags: tagNames==nil → keep; len==0 → remove all; len>0 → replace exactly with provided set.
func (crmdb *CRMDatabase) SaveCompany(c *Company, ownerID uint, tagNames []string) error {
	// Ownership guard: callers must pass the same owner scope as the record.
	if c.OwnerID != ownerID {
		return ErrNotAllowed
	}

	return crmdb.db.Transaction(func(tx *gorm.DB) error {
		// 1) Upsert company record (associations handled explicitly below)
		var err error
		if c.ID == 0 {
			if err = tx.Create(c).Error; err != nil {
				return err
			}
		} else {
			// Update a controlled set of fields within the owner scope.
			if err = tx.Model(&Company{}).Where("id = ? AND owner_id = ?", c.ID, ownerID).
				Updates(map[string]any{
					"address1":                 c.Address1,
					"address2":                 c.Address2,
					"background":               c.Background,
					"contact_invoice":          c.ContactInvoice,
					"default_tax_rate":         c.DefaultTaxRate,
					"invoice_currency":         c.InvoiceCurrency,
					"invoice_exemption_reason": c.InvoiceExemptionReason,
					"invoice_footer":           c.InvoiceFooter,
					"invoice_opening":          c.InvoiceOpening,
					"invoice_tax_type":         c.InvoiceTaxType,
					"customer_number":          c.CustomerNumber,
					"country":                  c.Country,
					"name":                     c.Name,
					"city":                     c.City,
					"zip":                      c.Zip,
					"invoice_email":            c.InvoiceEmail,
					"supplier_number":          c.SupplierNumber,
					"vat_id":                   c.VATID,
				}).Error; err != nil {
				return err
			}
		}

		// 2) Replace ContactInfos for this company (delete-all + insert provided slice)
		if err = tx.
			Where("owner_id = ? AND parent_type = ? AND parent_id = ?", ownerID, ParentTypeCompany, c.ID).
			Delete(&ContactInfo{}).Error; err != nil {
			return err
		}
		if len(c.ContactInfos) > 0 {
			// Ensure polymorphic linkage and owner scope on each row.
			for i := range c.ContactInfos {
				c.ContactInfos[i].OwnerID = ownerID
				c.ContactInfos[i].ParentType = ParentTypeCompany
				c.ContactInfos[i].ParentID = c.ID
			}
			if err = tx.Create(&c.ContactInfos).Error; err != nil {
				return err
			}
		}

		// 3) Tags: nil = leave; empty = remove all; non-empty = replace
		switch {
		case tagNames == nil:
			// leave as-is
		case len(tagNames) == 0:
			if err := tx.
				Where("owner_id = ? AND parent_type = ? AND parent_id = ?", ownerID, ParentTypeCompany, c.ID).
				Delete(&TagLink{}).Error; err != nil {
				return err
			}
		default:
			tags, err := crmdb.ensureTags(tx, ownerID, tagNames)
			if err != nil {
				return err
			}
			if err := crmdb.replaceTagsForParent(tx, ownerID, ParentTypeCompany, c.ID, tags); err != nil {
				return err
			}
		}

		return nil
	})
}

// LoadCompany loads a company by (id, ownerID), including:
//   - Invoices (ordered newest first),
//   - ContactInfos,
//   - Contacts (people) via a follow-up query.
func (crmdb *CRMDatabase) LoadCompany(id any, ownerID any) (*Company, error) {
	var err error
	if ownerID == nil {
		return nil, fmt.Errorf("userid is nil")
	}
	c := &Company{}
	result := crmdb.db.
		Preload("Invoices", func(db *gorm.DB) *gorm.DB {
			return db.Order("invoices.created_at DESC")
		}).
		Preload("ContactInfos").
		First(c, "owner_id = ? AND id = ?", ownerID, id)
	if result.Error != nil {
		return nil, result.Error
	}
	c.Contacts, err = crmdb.LoadPeopleForCompany(c.ID, ownerID)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// LoadAllCompanies returns all companies for a given owner, preloading ContactInfos.
// Use with care for large datasets (consider pagination).
func (crmdb *CRMDatabase) LoadAllCompanies(ownerid any) ([]*Company, error) {
	var companies = make([]*Company, 0)
	result := crmdb.db.Preload("ContactInfos").Where("owner_id = ?", ownerid).Find(&companies)
	return companies, result.Error
}

// likeEscape escapes wildcard and escape characters for SQL LIKE queries.
// It doubles backslashes and escapes '%' and '_' to avoid unintended matches.
func likeEscape(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

// FindAllCompaniesWithText performs a case-insensitive substring search on company names
// within an owner scope. Uses ILIKE on PostgreSQL and LOWER(name) LIKE on other dialects.
// ContactInfos are preloaded for convenience.
func (crmdb *CRMDatabase) FindAllCompaniesWithText(search string, ownerid uint) ([]*Company, error) {
	search = likeEscape(search)
	like := "%" + search + "%"
	var companies []*Company

	q := crmdb.db.Preload("ContactInfos")

	switch crmdb.db.Dialector.Name() {
	case "postgres":
		q = q.Where("owner_id = ? AND name ILIKE ? ESCAPE '\\'", ownerid, like)
	default: // sqlite, mysql/mariadb
		q = q.Where("owner_id = ? AND LOWER(name) LIKE LOWER(?) ESCAPE '\\'", ownerid, like)
	}

	err := q.Find(&companies).Error
	return companies, err
}

// CompanyNamesByIDs returns a map of company ID → company name for a given set of IDs.
// Efficiently implemented via a selective scan on the "companies" table.
func (crmdb *CRMDatabase) CompanyNamesByIDs(ownerID uint, ids []uint) (map[uint]string, error) {
	if len(ids) == 0 {
		return map[uint]string{}, nil
	}
	type row struct {
		ID   uint
		Name string
	}
	var rs []row
	if err := crmdb.db.
		Table("companies").
		Select("id, name").
		Where("owner_id = ? AND id IN ?", ownerID, ids).
		Scan(&rs).Error; err != nil {
		return nil, err
	}

	out := make(map[uint]string, len(rs))
	for _, r := range rs {
		out[r.ID] = r.Name
	}
	return out, nil
}

// ListAllCompaniesByTags returns all companies matching the given filters (no external pagination).
// Internally iterates in fixed-size pages to control memory usage.
func (crmdb *CRMDatabase) ListAllCompaniesByTags(ownerID uint, f CompanyListFilters) ([]Company, error) {
	// Reuse the same filtering logic; page internally
	const pageSize = 500
	var out []Company
	offset := 0

	for {
		page, err := crmdb.SearchCompaniesByTags(ownerID, CompanyListFilters{
			Query:   f.Query,
			Tags:    f.Tags,
			ModeAND: f.ModeAND,
			Limit:   pageSize,
			Offset:  offset,
		})
		if err != nil {
			return nil, err
		}
		out = append(out, page.Companies...)
		if len(page.Companies) < pageSize {
			break
		}
		offset += pageSize
	}
	return out, nil
}

// TagsForCompanies returns a map[companyID][]Tag for the given company IDs.
// Skips soft-deleted tag links and orders tags case-insensitively by name.
func (crmdb *CRMDatabase) TagsForCompanies(ownerID uint, ids []uint) (map[uint][]Tag, error) {
	if len(ids) == 0 {
		return map[uint][]Tag{}, nil
	}
	var rows []struct {
		CompanyID uint
		ID        uint
		Name      string
	}
	err := crmdb.db.
		Table("tag_links tl").
		Select("tl.parent_id AS company_id, t.id, t.name").
		Joins("JOIN tags t ON t.id = tl.tag_id").
		Where("tl.owner_id = ? AND tl.parent_type = ? AND tl.deleted_at IS NULL", ownerID, ParentTypeCompany).
		Where("tl.parent_id IN ?", ids).
		Order("LOWER(t.name) ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[uint][]Tag, len(ids))
	for _, r := range rows {
		out[r.CompanyID] = append(out[r.CompanyID], Tag{ID: r.ID, Name: r.Name})
	}
	return out, nil
}
