package model

import (
	"html/template"
	"time"

	"gorm.io/gorm"
)

// ContactInfo represents a single communication channel for a parent entity.
// It is polymorphic, meaning it can belong to either a Company or a Person.
//
// Examples:
//
//	Type:  "phone", "fax", "email", "website", "linkedin", "twitter", "github", "other"
//	Label: "Office", "Support", "Main Line"
//	Value: "+49 89 1234567", "info@example.com", "example.com"
//
// The combination (OwnerID, ParentType, ParentID) identifies the entity the
// contact belongs to. Records are soft-deletable via GORM's DeletedAt.
type ContactInfo struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	OwnerID    uint   `gorm:"index"`                            // Tenant or account owner
	ParentID   uint   `gorm:"index:idx_contact_parent"`         // Entity ID (company/person)
	ParentType string `gorm:"size:50;index:idx_contact_parent"` // "companies" | "people"

	Type  string `gorm:"size:30;index"` // Kind of contact info
	Label string `gorm:"size:100"`      // e.g. “Office”, “HQ”, “Support”
	Value string `gorm:"size:300"`      // Actual data (phone number, email, URL, etc.)
}

// Href returns a URI-ready representation of the contact info's value.
// It prepends a suitable scheme (e.g. tel:, mailto:, https://) depending on the Type.
//
// Examples:
//
//	Type=phone,  Value="012345"   → "tel:012345"
//	Type=email,  Value="a@b.com"  → "mailto:a@b.com"
//	Type=website, Value="foo.com" → "https://foo.com"
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
		// Fallback: attempt to treat as URL
		if hasScheme(c.Value) {
			return c.Value
		}
		return "https://" + c.Value
	}
}

// SafeHref returns a template.URL version of the Href() output for safe
// embedding in HTML templates.
func (c ContactInfo) SafeHref() template.URL {
	return template.URL(c.Href())
}

// OpensInNewTab indicates whether the contact info link should open in a new
// tab/window.
func (c ContactInfo) OpensInNewTab() bool {
	switch c.Type {
	case "website", "linkedin", "twitter", "github":
		return true
	default:
		return false
	}
}

// hasScheme detects whether a string already begins with a URI scheme.
// Recognized schemes: http://, https://, tel:, mailto:
func hasScheme(s string) bool {
	return len(s) > 7 && (s[:7] == "http://" || (len(s) > 8 && s[:8] == "https://")) ||
		(len(s) > 5 && (s[:5] == "tel:/" || s[:7] == "mailto:"))
}

// LoadPhone loads a ContactInfo entry (of any type, not strictly “phone”)
// by its primary key and owner ID.
//
// Returns the matching ContactInfo or a gorm.ErrRecordNotFound if not found.
func (crmdb *CRMDatabase) LoadPhone(phoneid any, ownerid any) (*ContactInfo, error) {
	ph := ContactInfo{}
	result := crmdb.db.Where("owner_id = ?", ownerid).First(&ph, phoneid)
	return &ph, result.Error
}

// DeletePhoneWithCompanyIDAndOwnerID deletes all ContactInfo records of type "companies"
// for a given company ID and owner ID. Used to clean up contacts on company deletion.
func (crmdb *CRMDatabase) DeletePhoneWithCompanyIDAndOwnerID(companyid any, ownerid any) error {
	ph := ContactInfo{}
	result := crmdb.db.Where("owner_id = ? AND parent_id = ? and parent_type = ?", ownerid, companyid, "companies").Delete(&ph)
	if err := result.Error; err != nil {
		return err
	}
	return nil
}

// DeletePhoneWithPersonIDAndOwnerID deletes all ContactInfo records linked to a specific person.
func (crmdb *CRMDatabase) DeletePhoneWithPersonIDAndOwnerID(personid any, ownerid any) error {
	ph := ContactInfo{}
	result := crmdb.db.Where("owner_id = ? AND parent_id = ? and parent_type = ?", ownerid, personid, "people").Delete(&ph)
	if err := result.Error; err != nil {
		return err
	}
	return nil
}

// DeletePhone deletes a single ContactInfo record by its ID.
//
// It first ensures the record exists, then performs a delete (soft delete if GORM
// soft deletes are active on this model). Returns an error if not found or failed.
func (crmdb *CRMDatabase) DeletePhone(id any) error {
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
