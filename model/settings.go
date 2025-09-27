package model

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Settings contains user data such as address.
// We keep gorm.Model (ID, timestamps) and make OwnerID UNIQUE.
// -> genau eine Settings-Zeile pro Owner.
type Settings struct {
	gorm.Model
	OwnerID               uint   `gorm:"uniqueIndex;column:owner_id"`
	CompanyName           string `gorm:"column:company_name"`
	InvoiceContact        string `gorm:"column:invoice_contact"`
	InvoiceEMail          string `gorm:"column:invoice_email"` // statt invoice_e_mail
	ZIP                   string `gorm:"column:zip"`
	Address1              string `gorm:"column:address1"`
	Address2              string `gorm:"column:address2"`
	City                  string `gorm:"column:city"`
	CountryCode           string `gorm:"column:country_code"`
	VATID                 string `gorm:"column:vat_id"`
	TAXNumber             string `gorm:"column:tax_number"`
	InvoiceNumberTemplate string `gorm:"column:invoice_number_template"`
	UseLocalCounter       bool   `gorm:"column:use_local_counter"`
	BankIBAN              string `gorm:"column:bank_iban"`
	BankName              string `gorm:"column:bank_name"`
	BankBIC               string `gorm:"column:bank_bic"`
}

// LoadSettings loads the settings for a given owner.
// Nutzt owner_id statt Primärschlüssel-ID.
func (crmdb *CRMDatenbank) LoadSettings(ownerID any) (*Settings, error) {
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
	// FirstOrInit: falls nicht vorhanden, mit OwnerID vorinitialisieren (noch nicht speichern)
	if err := crmdb.db.
		Where("owner_id = ?", oid).
		FirstOrInit(s, &Settings{OwnerID: oid}).Error; err != nil {
		return nil, err
	}
	return s, nil
}

// UpdateSettings updates fields for an existing row (identified by OwnerID).
// Nutzt WHERE owner_id, damit nicht versehentlich per primärem ID-Feld upgedatet wird.
func (crmdb *CRMDatenbank) UpdateSettings(s *Settings) error {
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

// SaveSettings upserts by owner_id (ON CONFLICT DO UPDATE).
func (crmdb *CRMDatenbank) SaveSettings(s *Settings) error {
	if s.OwnerID == 0 {
		return errors.New("SaveSettings: OwnerID required")
	}
	return crmdb.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "owner_id"}}, // Konfliktziel
		DoUpdates: clause.AssignmentColumns([]string{
			"company_name", "invoice_contact", "invoice_email",
			"zip", "address1", "address2", "city", "country_code",
			"vat_id", "tax_number", "invoice_number_template",
			"use_local_counter", "bank_iban", "bank_name", "bank_bic",
			"updated_at",
		}),
	}).Create(s).Error
}
