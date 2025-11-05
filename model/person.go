package model

import (
	"gorm.io/gorm"
)

// Person represents a natural person (human contact).
// It is owned by an account (OwnerID) and may be linked to a Company.
// Related ContactInfos and Notes are modeled via polymorphic associations.
type Person struct {
	gorm.Model
	OwnerID   uint   `gorm:"column:owner_id"` // Owning account (tenant) – used for scoping/authorization
	Name      string `gorm:"column:name"`
	Position  string `gorm:"column:position"` // Job title or role at the company
	EMail     string `gorm:"column:e_mail"`
	CompanyID int    `gorm:"column:company_id"`
	Company   Company
	// Polymorphic association: ContactInfos belong to various parent types (here: Person).
	ContactInfos []ContactInfo `gorm:"polymorphic:Parent;polymorphicValue:people"`
	// Notes are polymorphic and cascade on delete; removing a Person deletes its Notes.
	Notes []Note `gorm:"polymorphic:Parent;constraint:OnDelete:CASCADE;"`
}

// CreatePerson inserts or updates a Person and (optionally) replaces its tags.
//
// Behavior:
//   - If p.ID == 0: insert; else: update the whole record via Save(p).
//   - Tags: if tagNames is non-empty, tags are ensured/created and REPLACED on the person.
//     (If tagNames is empty, tags are left untouched.)
//   - Owner scoping: OwnerID is taken from the person record (p.OwnerID).
//
// Atomicity: Executed in a single DB transaction; either everything succeeds or nothing is written.
func (crmdb *CRMDatabase) CreatePerson(p *Person, tagNames []string) error {
	return crmdb.db.Transaction(func(tx *gorm.DB) error {
		var err error
		if p.ID == 0 {
			err = tx.Create(p).Error
		} else {
			err = tx.Save(p).Error
		}
		if err != nil {
			return err
		}

		// OwnerID is read from the person itself.
		if len(tagNames) > 0 {
			tags, err := crmdb.ensureTags(tx, p.OwnerID, tagNames)
			if err != nil {
				return err
			}
			// Replace to keep the source of truth at the call site.
			if err := crmdb.replaceTagsForParent(tx, p.OwnerID, ParentTypePerson, p.ID, tags); err != nil {
				return err
			}
		}
		return nil
	})
}

// SavePerson upserts a person, fully replaces its ContactInfos, and replaces its tags.
// Transactional and owner-scoped.
//
// Semantics:
//   - Upsert:
//   - New person (p.ID == 0): insert.
//   - Existing person: update whitelisted fields (name, e_mail, position, company_id)
//     but only within the given owner scope.
//   - ContactInfos: "replace" semantics. Existing rows are deleted, then the provided set is inserted.
//   - Tags:
//   - tagNames == nil  -> leave existing tags unchanged
//   - len(tagNames) == 0 -> remove all tags
//   - len(tagNames) > 0  -> replace with exactly these tags
//
// Security/scope: Operation is rejected if p.OwnerID != ownerID.
func (crmdb *CRMDatabase) SavePerson(p *Person, ownerID uint, tagNames []string) error {
	if p.OwnerID != ownerID {
		return ErrNotAllowed
	}

	return crmdb.db.Transaction(func(tx *gorm.DB) error {
		var err error
		if p.ID == 0 {
			if err = tx.Create(p).Error; err != nil {
				return err
			}
		} else {
			// Update only selected fields and only within the owner scope.
			if err = tx.Model(&Person{}).Where("id = ? AND owner_id = ?", p.ID, ownerID).
				Updates(map[string]any{
					"name":       p.Name,
					"e_mail":     p.EMail,
					"position":   p.Position,
					"company_id": p.CompanyID,
				}).Error; err != nil {
				return err
			}
		}

		// Replace ContactInfos: delete all, then insert provided set (if any).
		if err = tx.
			Where("owner_id = ? AND parent_type = ? AND parent_id = ?", ownerID, ParentTypePerson, p.ID).
			Delete(&ContactInfo{}).Error; err != nil {
			return err
		}
		if len(p.ContactInfos) > 0 {
			for i := range p.ContactInfos {
				p.ContactInfos[i].OwnerID = ownerID
				p.ContactInfos[i].ParentType = ParentTypePerson
				p.ContactInfos[i].ParentID = p.ID
			}
			if err = tx.Create(&p.ContactInfos).Error; err != nil {
				return err
			}
		}

		// Tags: nil = keep; empty = remove all; non-empty = replace
		switch {
		case tagNames == nil:
			// Keep tags as-is.
		case len(tagNames) == 0:
			if err := tx.
				Where("owner_id = ? AND parent_type = ? AND parent_id = ?", ownerID, ParentTypePerson, p.ID).
				Delete(&TagLink{}).Error; err != nil {
				return err
			}
		default:
			tags, err := crmdb.ensureTags(tx, ownerID, tagNames)
			if err != nil {
				return err
			}
			if err := crmdb.replaceTagsForParent(tx, ownerID, ParentTypePerson, p.ID, tags); err != nil {
				return err
			}
		}

		return nil
	})
}

// LoadPeopleForCompany returns all contacts for a given company within an owner scope.
// ContactInfos are preloaded.
func (crmdb *CRMDatabase) LoadPeopleForCompany(id any, ownerID any) ([]*Person, error) {
	var people = make([]*Person, 0)
	result := crmdb.db.Preload("ContactInfos").
		Where("owner_id = ?", ownerID).
		Where("company_id = ?", id).
		Find(&people)
	return people, result.Error
}

// RemovePerson deletes a person if it belongs to the given owner.
// Returns ErrNotAllowed when the owner check fails.
func (crmdb *CRMDatabase) RemovePerson(id any, ownerID any) error {
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

// LoadPerson fetches a person by id within an owner scope.
// Preloads ContactInfos and Company for convenience.
func (crmdb *CRMDatabase) LoadPerson(id any, ownerID any) (*Person, error) {
	c := &Person{}
	result := crmdb.db.Preload("ContactInfos").
		Preload("Company").
		Where("owner_id = ?", ownerID).
		First(c, id)
	return c, result.Error
}

// FindAllPeopleWithText performs a case-insensitive substring search on person names
// within an owner scope. Uses ILIKE on PostgreSQL; uses LOWER(name) LIKE on other dialects.
func (crmdb *CRMDatabase) FindAllPeopleWithText(search string, ownerid uint) ([]*Person, error) {
	search = likeEscape(search)
	like := "%" + search + "%"

	var people []*Person
	q := crmdb.db.Preload("ContactInfos")

	switch crmdb.db.Dialector.Name() {
	case "postgres":
		// PostgreSQL: ILIKE for case-insensitive search with explicit ESCAPE.
		q = q.Where("owner_id = ? AND name ILIKE ? ESCAPE '\\'", ownerid, like)
	default: // sqlite, mysql/mariadb
		// Generic fallback: LOWER(name) LIKE LOWER(?) with explicit ESCAPE.
		q = q.Where("owner_id = ? AND LOWER(name) LIKE LOWER(?) ESCAPE '\\'", ownerid, like)
	}

	err := q.Find(&people).Error
	return people, err
}
