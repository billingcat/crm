package model

import (
	"time"

	"gorm.io/gorm"
)

type ContactInfo struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	OwnerID    uint   `gorm:"index"`
	ParentID   int    `gorm:"index:idx_contact_parent"`
	ParentType string `gorm:"size:50;index:idx_contact_parent"`

	// 'phone','fax','email','website','linkedin','twitter','github','other', ...
	Type  string `gorm:"size:30;index"`
	Label string `gorm:"size:100"` // z.B. Büro, Zentrale, Support
	Value string `gorm:"size:300"` // eigentliche Nummer/URL/E-Mail
}

func (c ContactInfo) Href() string {
	switch c.Type {
	case "phone":
		return "tel:" + c.Value
	case "fax":
		return "fax:" + c.Value
	case "email":
		return "mailto:" + c.Value
	case "website", "linkedin", "twitter", "github":
		if hasScheme(c.Value) {
			return c.Value
		}
		return "https://" + c.Value
	default:
		// Fallback: versuche URL
		if hasScheme(c.Value) {
			return c.Value
		}
		return "https://" + c.Value
	}
}

func hasScheme(s string) bool {
	return len(s) > 7 && (s[:7] == "http://" || (len(s) > 8 && s[:8] == "https://")) ||
		(len(s) > 5 && (s[:5] == "tel:/" || s[:7] == "mailto:"))
}

// LoadPhone lädt eine Telefonnummer
func (crmdb *CRMDatenbank) LoadPhone(phoneid any, ownerid any) (*ContactInfo, error) {
	ph := ContactInfo{}
	result := crmdb.db.Where("owner_id = ?", ownerid).First(&ph, phoneid)
	return &ph, result.Error
}

func (crmdb *CRMDatenbank) DeletePhoneWithCompanyIDAndOwnerID(companyid any, ownerid any) error {
	ph := ContactInfo{}

	result := crmdb.db.Where("owner_id = ? AND parent_id = ? and parent_type = ?", ownerid, companyid, "companies").Delete(&ph)
	if err := result.Error; err != nil {
		return err
	}
	return nil
}

func (crmdb *CRMDatenbank) DeletePhoneWithPersonIDAndOwnerID(personid any, ownerid any) error {
	ph := ContactInfo{}

	result := crmdb.db.Where("owner_id = ? AND parent_id = ? and parent_type = ?", ownerid, personid, "people").Delete(&ph)
	if err := result.Error; err != nil {
		return err
	}
	return nil
}

// DeletePhone löscht die Telefonnummer mit der angegebenen ID
func (crmdb *CRMDatenbank) DeletePhone(id any) error {
	ph := ContactInfo{}
	result := crmdb.db.First(&ph, id)
	if err := result.Error; err != nil {
		return err
	}
	result = crmdb.db.Delete(&ph)
	if err := result.Error; err != nil {
		return err
	}
	return nil
}
