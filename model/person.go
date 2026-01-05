package model

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	// DepartedAt marks when the person left the company (nil = still active)
	DepartedAt *time.Time `gorm:"column:departed_at"`
	// Polymorphic association: ContactInfos belong to various parent types (here: Person).
	ContactInfos []ContactInfo `gorm:"polymorphic:Parent;polymorphicValue:person"`
	// Notes are polymorphic and cascade on delete; removing a Person deletes its Notes.
	Notes []Note `gorm:"polymorphic:Parent;polymorphicValue:person;constraint:OnDelete:CASCADE;"`
}

// HasDeparted returns true if the person has left the company
func (p *Person) HasDeparted() bool {
	return p.DepartedAt != nil
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
func (s *Store) CreatePerson(p *Person, tagNames []string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
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
			tags, err := s.ensureTags(tx, p.OwnerID, tagNames)
			if err != nil {
				return err
			}
			// Replace to keep the source of truth at the call site.
			if err := s.replaceTagsForParent(tx, p.OwnerID, ParentTypePerson, p.ID, tags); err != nil {
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
func (s *Store) SavePerson(p *Person, ownerID uint, tagNames []string) error {
	if p.OwnerID != ownerID {
		return ErrNotAllowed
	}

	// Capture ContactInfos to avoid GORM auto-saving associations on Create.
	contactInfos := p.ContactInfos
	p.ContactInfos = nil

	return s.db.Transaction(func(tx *gorm.DB) error {
		var err error
		if p.ID == 0 {
			// Skip auto-saving associations; we handle ContactInfos manually below.
			if err = tx.Omit(clause.Associations).Create(p).Error; err != nil {
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
		if len(contactInfos) > 0 {
			for i := range contactInfos {
				contactInfos[i].OwnerID = ownerID
				contactInfos[i].ParentType = ParentTypePerson
				contactInfos[i].ParentID = p.ID
			}
			if err = tx.Create(&contactInfos).Error; err != nil {
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
			tags, err := s.ensureTags(tx, ownerID, tagNames)
			if err != nil {
				return err
			}
			if err := s.replaceTagsForParent(tx, ownerID, ParentTypePerson, p.ID, tags); err != nil {
				return err
			}
		}

		return nil
	})
}

// LoadPeopleForCompany returns all contacts for a given company within an owner scope.
// ContactInfos are preloaded.
func (s *Store) LoadPeopleForCompany(id any, ownerID any) ([]*Person, error) {
	var people = make([]*Person, 0)
	result := s.db.Preload("ContactInfos").
		Where("owner_id = ?", ownerID).
		Where("company_id = ?", id).
		Find(&people)
	return people, result.Error
}

// RemovePerson deletes a person if it belongs to the given owner.
// Returns ErrNotAllowed when the owner check fails.
func (s *Store) RemovePerson(id any, ownerID any) error {
	var owner uint
	var ok bool
	if owner, ok = ownerID.(uint); !ok {
		return ErrNotAllowed
	}
	c := &Person{}
	result := s.db.Where("owner_id = ?", owner).First(c, id)
	if result.Error != nil {
		return result.Error
	}
	if c.OwnerID != owner {
		return ErrNotAllowed
	}
	result = s.db.Delete(c)
	return result.Error
}

// LoadPerson fetches a person by id within an owner scope.
// Preloads ContactInfos and Company for convenience.
func (s *Store) LoadPerson(id any, ownerID any) (*Person, error) {
	c := &Person{}
	result := s.db.Preload("ContactInfos").
		Preload("Company").
		Where("owner_id = ?", ownerID).
		First(c, id)
	return c, result.Error
}

// FindAllPeopleWithText performs a case-insensitive substring search on person names
// within an owner scope. Uses ILIKE on PostgreSQL; uses LOWER(name) LIKE on other dialects.
func (s *Store) FindAllPeopleWithText(search string, ownerid uint) ([]*Person, error) {
	search = likeEscape(search)
	like := "%" + search + "%"

	var people []*Person
	q := s.db.Preload("ContactInfos")

	switch s.db.Dialector.Name() {
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

// ListPersonsForExportCtx returns all persons for the given owner,
// preloading ContactInfos and Notes.
// Used for data export.
func (s *Store) ListPersonsForExportCtx(
	ctx context.Context,
	ownerID uint,
) ([]Person, error) {
	var persons []Person

	q := s.db.WithContext(ctx).
		Where("owner_id = ?", ownerID).
		Preload("ContactInfos").
		Preload("Notes").
		Order("id ASC")

	if err := q.Find(&persons).Error; err != nil {
		return nil, fmt.Errorf("list persons for export (owner %d): %w", ownerID, err)
	}

	return persons, nil
}

// DepartPersonResult contains the result of a DepartPerson operation
type DepartPersonResult struct {
	Person *Person
	Note   *Note
}

// DepartPerson marks a person as having left their company and creates a note on the company.
//
// This operation:
//   - Sets DepartedAt to the current time
//   - Creates a note on the associated company documenting the departure
//   - The person remains linked to the company for historical reference
//
// Parameters:
//   - personID: ID of the person departing
//   - ownerID: owner scope for authorization
//   - authorID: user creating the departure record (for the note)
//
// Returns ErrNotAllowed if the person doesn't belong to the owner.
// Returns an error if the person has no associated company.
func (s *Store) DepartPerson(personID uint, ownerID uint, authorID uint) (*DepartPersonResult, error) {
	var result DepartPersonResult

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Load person with company
		var person Person
		if err := tx.Preload("Company").
			Where("id = ? AND owner_id = ?", personID, ownerID).
			First(&person).Error; err != nil {
			return err
		}

		// Check if already departed
		if person.DepartedAt != nil {
			return fmt.Errorf("person has already departed")
		}

		// Check if person has a company
		if person.CompanyID == 0 {
			return fmt.Errorf("person has no associated company")
		}

		// Set departure time
		now := time.Now()
		person.DepartedAt = &now

		if err := tx.Model(&Person{}).
			Where("id = ? AND owner_id = ?", personID, ownerID).
			Update("departed_at", now).Error; err != nil {
			return err
		}

		// Create note on the company
		noteBody := fmt.Sprintf("%s ist am %s ausgeschieden.",
			person.Name,
			now.Format("02.01.2006"))

		note := &Note{
			OwnerID:    ownerID,
			AuthorID:   authorID,
			ParentType: ParentTypeCompany,
			ParentID:   uint(person.CompanyID),
			Title:      "Mitarbeiter ausgeschieden",
			Body:       noteBody,
			Tags:       "personal,ausgeschieden",
		}

		if err := tx.Create(note).Error; err != nil {
			return err
		}

		result.Person = &person
		result.Note = note
		return nil
	})

	if err != nil {
		return nil, err
	}
	return &result, nil
}

// ReactivatePerson removes the departed status from a person.
// Use this if a person returns to the company or was marked departed by mistake.
func (s *Store) ReactivatePerson(personID uint, ownerID uint) error {
	return s.db.Model(&Person{}).
		Where("id = ? AND owner_id = ?", personID, ownerID).
		Update("departed_at", nil).Error
}
