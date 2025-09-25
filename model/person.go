package model

import (
	"gorm.io/gorm"
)

// A Person is a natural identity
type Person struct {
	gorm.Model
	OwnerID   uint
	Name      string
	Position  string
	EMail     string
	CompanyID int
	Company   Company
	Phones    []Phone `gorm:"polymorphic:Parent;"`
	Notes     []Note  `gorm:"polymorphic:Parent;constraint:OnDelete:CASCADE;"`
}

// CreatePerson person
func (crmdb *CRMDatenbank) CreatePerson(p *Person) error {
	if p.ID == 0 {
		result := crmdb.db.Create(p)
		return result.Error
	}
	result := crmdb.db.Save(p)
	return result.Error
}

// SavePerson saves a Person
func (crmdb *CRMDatenbank) SavePerson(p *Person, uid any) error {
	var owner uint
	var ok bool
	if owner, ok = uid.(uint); !ok {
		return ErrNotAllowed
	}
	if p.OwnerID != owner {
		return ErrNotAllowed
	}
	crmdb.DeletePhoneWithPersonIDAndOwnerID(p.ID, p.OwnerID)
	result := crmdb.db.Save(p)
	return result.Error
}

// LoadPeopleForCompany loads all contacts for a company
func (crmdb *CRMDatenbank) LoadPeopleForCompany(id any, ownerID any) ([]*Person, error) {
	var people = make([]*Person, 0)
	result := crmdb.db.Preload("Phones").Where("owner_id = ?", ownerID).Where("company_id = ?", id).Find(&people)
	return people, result.Error
}

// Remove a person
func (crmdb *CRMDatenbank) RemovePerson(id any, ownerID any) error {
	var owner uint
	var ok bool
	if owner, ok = ownerID.(uint); !ok {
		return ErrNotAllowed
	}
	c := &Person{}
	result := crmdb.db.Where("owner_id = ?", owner).First(c, id)
	if result.Error != nil {
		return result.Error
	}
	if c.OwnerID != owner {
		return ErrNotAllowed
	}
	result = crmdb.db.Delete(c)
	return result.Error
}

// LoadPerson loads a Person
func (crmdb *CRMDatenbank) LoadPerson(id any, ownerID any) (*Person, error) {
	c := &Person{}
	result := crmdb.db.Preload("Phones").Preload("Company").Where("owner_id = ?", ownerID).First(c, id)
	return c, result.Error
}

// FindAllPeopleWithText sucht über alle Personendaten eines Owners.
func (crmdb *CRMDatenbank) FindAllPeopleWithText(search string, ownerid uint) ([]*Person, error) {
	var people []*Person
	result := crmdb.db.
		Preload("Phones").
		Where("owner_id = ? AND name LIKE ?", ownerid, "%"+search+"%").
		Find(&people)

	return people, result.Error
}
