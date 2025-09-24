package model

import (
	"time"
)

// Phone ist eine Telefonnummer, Faxnummer etc
type Phone struct {
	ID         uint `gorm:"primarykey"`
	CreatedAt  time.Time
	OwnerID    uint
	ParentID   int
	ParentType string
	Number     string
	Location   string
}

// LoadPhone lädt eine Telefonnummer
func (crmdb *CRMDatenbank) LoadPhone(phoneid any, ownerid any) (*Phone, error) {
	ph := Phone{}
	result := crmdb.db.Where("owner_id = ?", ownerid).First(&ph, phoneid)
	return &ph, result.Error
}

func (crmdb *CRMDatenbank) DeletePhoneWithCompanyIDAndOwnerID(companyid any, ownerid any) error {
	ph := Phone{}

	result := crmdb.db.Where("owner_id = ? AND parent_id = ? and parent_type = ?", ownerid, companyid, "companies").Delete(&ph)
	if err := result.Error; err != nil {
		return err
	}
	return nil
}

func (crmdb *CRMDatenbank) DeletePhoneWithPersonIDAndOwnerID(personid any, ownerid any) error {
	ph := Phone{}

	result := crmdb.db.Where("owner_id = ? AND parent_id = ? and parent_type = ?", ownerid, personid, "people").Delete(&ph)
	if err := result.Error; err != nil {
		return err
	}
	return nil
}

// DeletePhone löscht die Telefonnummer mit der angegebenen ID
func (crmdb *CRMDatenbank) DeletePhone(id any) error {
	ph := Phone{}
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
