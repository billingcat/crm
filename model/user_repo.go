// user_repo.go (Model snippet)
package model

import (
	"strings"
)

// ListUsers returns a page of users filtered by query `q` (matches email or full name, case-insensitive).
// It also returns the total count for pagination.
func (crmdb *CRMDatenbank) ListUsers(q string, offset, limit int) ([]User, int64, error) {
	var (
		users []User
		total int64
	)

	db := crmdb.db.Model(&User{})

	if q != "" {
		like := "%" + strings.ToLower(q) + "%"
		// NOTE: make sure you have LOWER() function support (Postgres ok; for SQLite use collations or custom).
		db = db.Where("LOWER(email) LIKE ? OR LOWER(full_name) LIKE ?", like, like)
	}

	// Count first
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Page data
	if err := db.Order("created_at DESC").Offset(offset).Limit(limit).Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}
