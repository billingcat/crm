package model

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Settings holds tenant-scoped account data such as address, invoicing details,
// and bank information. We keep the embedded gorm.Model (ID + timestamps) but
// enforce a UNIQUE owner_id so there is at most one settings row per owner.
type Settings struct {
	gorm.Model
	OwnerID               uint   `gorm:"uniqueIndex;column:owner_id"` // One row per owner/tenant
	CompanyName           string `gorm:"column:company_name"`
	InvoiceContact        string `gorm:"column:invoice_contact"`
	InvoiceEMail          string `gorm:"column:invoice_email"` // stored as invoice_email (not invoice_e_mail)
	ZIP                   string `gorm:"column:zip"`
	Address1              string `gorm:"column:address1"`
	Address2              string `gorm:"column:address2"`
	City                  string `gorm:"column:city"`
	CountryCode           string `gorm:"column:country_code"` // ISO 3166-1 alpha-2 recommended
	VATID                 string `gorm:"column:vat_id"`
	TAXNumber             string `gorm:"column:tax_number"`
	InvoiceNumberTemplate string `gorm:"column:invoice_number_template"` // e.g. "INV-{YYYY}-{NNNN}"
	UseLocalCounter       bool   `gorm:"column:use_local_counter"`       // if true, number increments per owner locally
	BankIBAN              string `gorm:"column:bank_iban"`
	BankName              string `gorm:"column:bank_name"`
	BankBIC               string `gorm:"column:bank_bic"`
}

// LoadSettings loads the settings row for a given owner.
// Accepts ownerID as uint or int and returns an initialized (but unsaved)
// Settings record if none exists yet (via FirstOrInit).
func (crmdb *CRMDatabase) LoadSettings(ownerID any) (*Settings, error) {
	var oid uint
	switch v := ownerID.(type) {
	case uint:
		oid = v
	case int:
		oid = uint(v)
	default:
		return nil, fmt.Errorf("LoadSettings: unsupported ownerID type %T", ownerID)
	}

	s := &Settings{}
	// FirstOrInit: if no row exists, return a struct prefilled with OwnerID
	// without hitting INSERT (use Save/SaveSettings later to persist).
	if err := crmdb.db.
		Where("owner_id = ?", oid).
		FirstOrInit(s, &Settings{OwnerID: oid}).Error; err != nil {
		return nil, err
	}
	return s, nil
}

// UpdateSettings updates fields for the existing row identified by owner_id.
// Uses an explicit WHERE owner_id filter to avoid accidentally updating by the
// primary key (ID) if the struct carries a different ID value.
//
// Note: updated_at uses NOW() which is DB-specific; for SQLite you may prefer
// CURRENT_TIMESTAMP or let GORM manage timestamps automatically.
func (crmdb *CRMDatabase) UpdateSettings(s *Settings) error {
	if s.OwnerID == 0 {
		return errors.New("UpdateSettings: OwnerID required")
	}
	return crmdb.db.
		Model(&Settings{}).
		Where("owner_id = ?", s.OwnerID).
		Updates(map[string]any{
			"company_name":            s.CompanyName,
			"invoice_contact":         s.InvoiceContact,
			"invoice_email":           s.InvoiceEMail,
			"zip":                     s.ZIP,
			"address1":                s.Address1,
			"address2":                s.Address2,
			"city":                    s.City,
			"country_code":            s.CountryCode,
			"vat_id":                  s.VATID,
			"tax_number":              s.TAXNumber,
			"invoice_number_template": s.InvoiceNumberTemplate,
			"use_local_counter":       s.UseLocalCounter,
			"bank_iban":               s.BankIBAN,
			"bank_name":               s.BankName,
			"bank_bic":                s.BankBIC,
			"updated_at":              gorm.Expr("NOW()"),
		}).Error
}

// SaveSettings performs an upsert keyed by owner_id (ON CONFLICT DO UPDATE).
// If a row for owner_id exists, the listed columns are updated; otherwise, a new
// row is inserted.
//
// Caveat: GORM translates ON CONFLICT per dialect. Ensure a unique index exists
// on owner_id (declared on the struct) and that the target DB supports the clause.
func (crmdb *CRMDatabase) SaveSettings(s *Settings) error {
	if s.OwnerID == 0 {
		return errors.New("SaveSettings: OwnerID required")
	}
	return crmdb.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "owner_id"}}, // conflict target
		DoUpdates: clause.AssignmentColumns([]string{
			"company_name", "invoice_contact", "invoice_email",
			"zip", "address1", "address2", "city", "country_code",
			"vat_id", "tax_number", "invoice_number_template",
			"use_local_counter", "bank_iban", "bank_name", "bank_bic",
			"updated_at",
		}),
	}).Create(s).Error
}
