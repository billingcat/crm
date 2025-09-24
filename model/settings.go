package model

import "gorm.io/gorm"

// Settings contains user data such as address.
type Settings struct {
	gorm.Model
	CompanyName           string
	InvoiceContact        string
	InvoiceEMail          string
	ZIP                   string
	Address1              string
	Address2              string
	City                  string
	CountryCode           string
	VATID                 string
	TAXNumber             string
	InvoiceNumberTemplate string
	UseLocalCounter       bool
	BankIBAN              string
	BankName              string
	BankBIC               string
}

// LoadSettings loads the user settings.
func (crmdb *CRMDatenbank) LoadSettings(userid any) (*Settings, error) {
	c := &Settings{}
	result := crmdb.db.FirstOrInit(c, userid)
	return c, result.Error
}

// UpdateSettings updates the user's settings.
func (crmdb *CRMDatenbank) UpdateSettings(s *Settings) error {
	return crmdb.db.Model(s).Updates(s).Error
}

// SaveSettings saves the user's settings.
func (crmdb *CRMDatenbank) SaveSettings(s *Settings) error {
	return crmdb.db.Save(s).Error
}
