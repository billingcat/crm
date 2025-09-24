package model

import (
	"fmt"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Company is a legal entity.
type Company struct {
	gorm.Model
	Adresse1               string
	Adresse2               string
	Background             string
	ContactInvoice         string
	DefaultTaxRate         decimal.Decimal `sql:"type:decimal(20,8);"`
	InvoiceCurrency        string
	InvoiceExemptionReason string
	InvoiceFooter          string
	InvoiceOpening         string
	Invoices               []Invoice
	InvoiceTaxType         string
	Kundennummer           string
	Land                   string
	Name                   string
	Ort                    string
	OwnerID                uint
	Phones                 []Phone   `gorm:"polymorphic:Parent;"`
	Contacts               []*Person `gorm:"-"`
	PLZ                    string
	RechnungEmail          string
	SupplierNumber         string
	VATID                  string
	Notes                  []Note `gorm:"polymorphic:Parent;constraint:OnDelete:CASCADE;"`
}

var ErrNotAllowed = fmt.Errorf("not allowed")

// SaveCompany speichert eine Firma.
func (crmdb *CRMDatenbank) SaveCompany(c *Company, userid any) error {
	var uid uint
	var ok bool
	if uid, ok = userid.(uint); !ok {
		return ErrNotAllowed
	}
	if c.OwnerID != uid {
		return ErrNotAllowed
	}
	crmdb.DeletePhoneWithCompanyIDAndOwnerID(c.ID, c.OwnerID)
	result := crmdb.db.Save(c)
	return result.Error
}

// LoadCompany lädt eine Firma und die dazugehörigen Rechnungen und
// Telefonnummern.
func (crmdb *CRMDatenbank) LoadCompany(id any, ownerID any) (*Company, error) {
	var err error
	if ownerID == nil {
		return nil, fmt.Errorf("userid is nil")
	}
	c := &Company{}
	result := crmdb.db.Preload("Invoices").Preload("Phones").Where("owner_id = ?", ownerID).First(c, id)
	if result.Error != nil {
		return nil, result.Error
	}
	c.Contacts, err = crmdb.LoadPeopleForCompany(c.ID, ownerID)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (crmdb *CRMDatenbank) LoadAllCompanies(ownerid any) ([]*Company, error) {
	var companies = make([]*Company, 0)
	result := crmdb.db.Preload("Phones").Where("owner_id = ?", ownerid).Find(&companies)
	return companies, result.Error
}

// FindAllCompaniesWithText sucht über alle Firmendaten.
func (crmdb *CRMDatenbank) FindAllCompaniesWithText(search string) ([]*Company, error) {
	var companies = make([]*Company, 0)
	result := crmdb.db.Preload("Phones").Where("name LIKE ?", "%"+search+"%").Find(&companies)
	return companies, result.Error
}
