package model

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

type Invitation struct {
	ID        uint   `gorm:"primaryKey"`
	Token     string `gorm:"uniqueIndex"`
	Email     string
	ExpiresAt *time.Time
	CreatedAt time.Time
}

// CreateInvitation inserts a new invitation into the database.
func (crmdb *CRMDatabase) CreateInvitation(ctx context.Context, inv *Invitation) error {
	// Ensure CreatedAt is present if not set by caller
	if inv.CreatedAt.IsZero() {
		inv.CreatedAt = time.Now()
	}

	if err := crmdb.db.WithContext(ctx).Create(inv).Error; err != nil {
		return err
	}

	return nil
}

// FindInvitationByToken looks up an invitation by its token.
// - Returns (nil, nil) if no invitation exists for the token.
// - Returns (*Invitation, nil) on success.
// - Returns a non-nil error for database errors.
func (crmdb *CRMDatabase) FindInvitationByToken(ctx context.Context, token string) (*Invitation, error) {
	// Normalize input early
	token = strings.TrimSpace(token)
	if token == "" {
		// Treat empty token as "no invitation"
		return nil, nil
	}

	var inv Invitation
	err := crmdb.db.WithContext(ctx).
		Where("token = ?", token).
		First(&inv).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Not found is not an error for callers; they can treat nil as "no invitation"
			return nil, nil
		}
		// Real database error
		return nil, err
	}

	return &inv, nil
}

// ListInvitations returns all invitations ordered by creation time (newest first).
func (crmdb *CRMDatabase) ListInvitations(ctx context.Context) ([]Invitation, error) {
	var invitations []Invitation

	if err := crmdb.db.WithContext(ctx).
		Order("created_at DESC").
		Find(&invitations).Error; err != nil {
		return nil, err
	}

	return invitations, nil
}
