package model

import (
	"fmt"
	"strings"

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
	ContactInfos           []ContactInfo `gorm:"polymorphic:Parent;"`
	Contacts               []*Person     `gorm:"-"`
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

func (crmdb *CRMDatenbank) LoadAllCompanies(ownerid any) ([]*Company, error) {
	var companies = make([]*Company, 0)
	result := crmdb.db.Preload("ContactInfos").Where("owner_id = ?", ownerid).Find(&companies)
	return companies, result.Error
}

func likeEscape(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

func (crmdb *CRMDatenbank) FindAllCompaniesWithText(search string, ownerid uint) ([]*Company, error) {
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

// In deinem model/Repo-Paket:
func (crmdb *CRMDatenbank) CompanyNamesByIDs(ownerID uint, ids []uint) (map[uint]string, error) {
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
