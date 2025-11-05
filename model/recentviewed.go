package model

import (
	"sort"
	"time"

	"gorm.io/gorm/clause"
)

type EntityType string

const (
	EntityCompany EntityType = "company"
	EntityPerson  EntityType = "person"
)

type RecentView struct {
	UserID     uint       `gorm:"not null;index:idx_user_view,priority:1"`
	EntityType EntityType `gorm:"type:text;not null;index:idx_user_view,priority:2"`
	EntityID   uint       `gorm:"not null;index:idx_user_view,priority:3"`
	ViewedAt   time.Time  `gorm:"not null;index:idx_user_viewed_at,priority:2"`
}

// Composite unique key
func (RecentView) TableName() string { return "recent_views" }

func (crmdb *CRMDatabase) TouchRecentView(userID uint, et EntityType, entityID uint) error {
	db := crmdb.db
	rv := RecentView{
		UserID: userID, EntityType: et, EntityID: entityID, ViewedAt: time.Now(),
	}
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "entity_type"}, {Name: "entity_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"viewed_at"}),
	}).Create(&rv).Error
}

type RecentItem struct {
	EntityType EntityType
	EntityID   uint
	ViewedAt   time.Time
	Name       string // Firmenname oder Personenname
}

func (crmdb *CRMDatabase) GetRecentItems(userID uint, limit int) ([]RecentItem, error) {
	db := crmdb.db
	items := []RecentItem{}

	var companies []RecentItem
	if err := db.Raw(`
        SELECT r.entity_type, r.entity_id, r.viewed_at, c.name
        FROM recent_views r
        JOIN companies c ON c.id = r.entity_id
        WHERE r.user_id = ? AND r.entity_type = 'company'
        ORDER BY r.viewed_at DESC
        LIMIT ?`, userID, limit).Scan(&companies).Error; err != nil {
		return nil, err
	}

	var people []RecentItem
	if err := db.Raw(`
        SELECT r.entity_type, r.entity_id, r.viewed_at,
               COALESCE(NULLIF(TRIM(p.name), ''), p.e_mail, 'Unbenannt') AS name
        FROM recent_views r
        JOIN people p ON p.id = r.entity_id
        WHERE r.user_id = ? AND r.entity_type = 'person'
        ORDER BY r.viewed_at DESC
        LIMIT ?`, userID, limit).Scan(&people).Error; err != nil {
		return nil, err
	}

	// merge and sort by ViewedAt descending
	items = append(companies, people...)
	sort.Slice(items, func(i, j int) bool { return items[i].ViewedAt.After(items[j].ViewedAt) })
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}
