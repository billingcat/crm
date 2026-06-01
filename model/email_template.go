package model

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// EmailTemplateKind identifies the message that a template configures
// (currently only invoice mails; further kinds like reminders can be added).
type EmailTemplateKind string

const (
	EmailTemplateKindInvoice EmailTemplateKind = "invoice"
)

// EmailTemplate stores a customizable mail subject + body.
//
// CompanyID == 0 means the row is the owner-wide default for the given kind.
// CompanyID > 0 means the row overrides the default for a specific company.
type EmailTemplate struct {
	ID        uint              `gorm:"primaryKey"`
	CreatedAt time.Time         `gorm:"not null"`
	UpdatedAt time.Time         `gorm:"not null"`
	OwnerID   uint              `gorm:"not null;uniqueIndex:idx_email_templates_unique,priority:1"`
	CompanyID uint              `gorm:"not null;default:0;uniqueIndex:idx_email_templates_unique,priority:2"`
	Kind      EmailTemplateKind `gorm:"type:text;not null;uniqueIndex:idx_email_templates_unique,priority:3"`
	Subject   string            `gorm:"type:text;not null;default:''"`
	Body      string            `gorm:"type:text;not null;default:''"`
}

func (EmailTemplate) TableName() string { return "email_templates" }

// LoadEmailTemplate returns the effective template for a given company and kind.
// Lookup order: company-specific override -> owner-wide default -> (nil, nil).
//
// Callers fall back to a hard-coded default when nil is returned.
func (s *Store) LoadEmailTemplate(ownerID, companyID uint, kind EmailTemplateKind) (*EmailTemplate, error) {
	if companyID != 0 {
		t, err := s.loadEmailTemplateRow(ownerID, companyID, kind)
		if err != nil {
			return nil, err
		}
		if t != nil {
			return t, nil
		}
	}
	return s.loadEmailTemplateRow(ownerID, 0, kind)
}

// LoadOwnerEmailTemplate returns the owner-wide default for a kind (no company fallback).
func (s *Store) LoadOwnerEmailTemplate(ownerID uint, kind EmailTemplateKind) (*EmailTemplate, error) {
	return s.loadEmailTemplateRow(ownerID, 0, kind)
}

// LoadCompanyEmailTemplate returns the company-specific override for a kind, or nil if none exists.
func (s *Store) LoadCompanyEmailTemplate(ownerID, companyID uint, kind EmailTemplateKind) (*EmailTemplate, error) {
	if companyID == 0 {
		return nil, errors.New("LoadCompanyEmailTemplate: companyID required")
	}
	return s.loadEmailTemplateRow(ownerID, companyID, kind)
}

func (s *Store) loadEmailTemplateRow(ownerID, companyID uint, kind EmailTemplateKind) (*EmailTemplate, error) {
	var t EmailTemplate
	err := s.db.
		Where("owner_id = ? AND company_id = ? AND kind = ?", ownerID, companyID, string(kind)).
		First(&t).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

// SaveEmailTemplate upserts a template row keyed by (owner_id, company_id, kind).
// If both Subject and Body are empty the row is deleted instead, so callers can
// "clear" a company override or wipe a global template.
func (s *Store) SaveEmailTemplate(t *EmailTemplate) error {
	if t.OwnerID == 0 {
		return errors.New("SaveEmailTemplate: OwnerID required")
	}
	if t.Kind == "" {
		return errors.New("SaveEmailTemplate: Kind required")
	}
	if t.Subject == "" && t.Body == "" {
		return s.deleteEmailTemplate(t.OwnerID, t.CompanyID, t.Kind)
	}
	return s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "owner_id"}, {Name: "company_id"}, {Name: "kind"},
		},
		DoUpdates: clause.Assignments(map[string]any{
			"subject":    t.Subject,
			"body":       t.Body,
			"updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
		}),
	}).Create(t).Error
}

func (s *Store) deleteEmailTemplate(ownerID, companyID uint, kind EmailTemplateKind) error {
	return s.db.
		Where("owner_id = ? AND company_id = ? AND kind = ?", ownerID, companyID, string(kind)).
		Delete(&EmailTemplate{}).Error
}
