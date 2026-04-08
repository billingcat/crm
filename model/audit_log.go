package model

import (
	"time"

	"gorm.io/gorm"
)

// AuditAction describes the type of action performed.
type AuditAction string

const (
	AuditActionCreate AuditAction = "create"
	AuditActionUpdate AuditAction = "update"
	AuditActionDelete AuditAction = "delete"
	AuditActionLogin  AuditAction = "login"
	AuditActionStatus AuditAction = "status" // e.g. invoice issued/paid/voided
)

// AuditEntityType describes the entity type affected.
type AuditEntityType string

const (
	AuditEntityCompany AuditEntityType = "company"
	AuditEntityPerson  AuditEntityType = "person"
	AuditEntityInvoice AuditEntityType = "invoice"
	AuditEntityNote    AuditEntityType = "note"
	AuditEntityUser    AuditEntityType = "user"
)

// AuditLog records a user action for the admin activity overview.
type AuditLog struct {
	ID         uint            `gorm:"primaryKey"`
	CreatedAt  time.Time       `gorm:"index;not null"`
	OwnerID    uint            `gorm:"not null;index"`
	UserID     uint            `gorm:"not null;index"`
	Action     AuditAction     `gorm:"type:text;not null"`
	EntityType AuditEntityType `gorm:"type:text;not null"`
	EntityID   uint            `gorm:"not null"`
	Summary    string          `gorm:"type:text"` // human-readable short description
}

func (AuditLog) TableName() string { return "audit_logs" }

// CreateAuditLog persists a single audit entry.
func (s *Store) CreateAuditLog(entry *AuditLog) error {
	return s.db.Create(entry).Error
}

// LogAudit is a convenience wrapper for creating audit entries.
func (s *Store) LogAudit(ownerID, userID uint, action AuditAction, entityType AuditEntityType, entityID uint, summary string) {
	_ = s.CreateAuditLog(&AuditLog{
		OwnerID:    ownerID,
		UserID:     userID,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		Summary:    summary,
	})
}

// AuditLogFilter holds optional filters for querying audit logs.
type AuditLogFilter struct {
	UserID     *uint
	Action     *AuditAction
	EntityType *AuditEntityType
}

// AuditLogEntry is a joined result including the user's name.
type AuditLogEntry struct {
	AuditLog
	UserEmail    string `gorm:"column:user_email"`
	UserFullName string `gorm:"column:user_full_name"`
}

// ListAuditLogs returns paginated audit log entries for an owner, newest first.
func (s *Store) ListAuditLogs(ownerID uint, filter AuditLogFilter, offset, limit int) ([]AuditLogEntry, int64, error) {
	if limit <= 0 {
		limit = 50
	}

	base := s.db.Table("audit_logs").
		Select("audit_logs.*, users.email AS user_email, users.full_name AS user_full_name").
		Joins("LEFT JOIN users ON users.id = audit_logs.user_id").
		Where("audit_logs.owner_id = ?", ownerID)

	if filter.UserID != nil {
		base = base.Where("audit_logs.user_id = ?", *filter.UserID)
	}
	if filter.Action != nil {
		base = base.Where("audit_logs.action = ?", *filter.Action)
	}
	if filter.EntityType != nil {
		base = base.Where("audit_logs.entity_type = ?", *filter.EntityType)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var entries []AuditLogEntry
	err := base.
		Order("audit_logs.created_at DESC").
		Offset(offset).
		Limit(limit).
		Scan(&entries).Error

	return entries, total, err
}

// ListAuditLogUsers returns distinct users who have audit log entries for the given owner.
func (s *Store) ListAuditLogUsers(ownerID uint) ([]User, error) {
	var users []User
	err := s.db.
		Where("id IN (?)",
			s.db.Table("audit_logs").
				Select("DISTINCT user_id").
				Where("owner_id = ?", ownerID),
		).
		Order("full_name ASC").
		Find(&users).Error
	return users, err
}

// AutoMigrateAuditLogs ensures the audit_logs table exists (used in dev/test).
func (s *Store) AutoMigrateAuditLogs() error {
	return s.db.AutoMigrate(&AuditLog{})
}

// PruneAuditLogs deletes audit log entries older than the given duration.
func (s *Store) PruneAuditLogs(ownerID uint, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	result := s.db.Where("owner_id = ? AND created_at < ?", ownerID, cutoff).Delete(&AuditLog{})
	return result.RowsAffected, result.Error
}

// CreateAuditLogInTx persists a single audit entry within an existing transaction.
func CreateAuditLogInTx(tx *gorm.DB, entry *AuditLog) error {
	return tx.Create(entry).Error
}
