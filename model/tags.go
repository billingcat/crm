// file: src/go/model/models_tags.go
package model

import (
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

/*
Tag represents a reusable label. We store both the display name (Name)
and a normalized version (Norm) to enforce cross-DB case-insensitive uniqueness.
Uniqueness is: (OwnerID, Norm).
*/
type Tag struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	// Composite index for lookups AND part of the unique constraint with Norm
	OwnerID uint   `gorm:"index:idx_tag_owner_name,priority:1;uniqueIndex:uniq_tag_per_owner,priority:1"`
	Name    string `gorm:"size:128;not null;index:idx_tag_owner_name,priority:2"`

	// Normalized tag for cross-DB case-insensitive uniqueness
	Norm string `gorm:"size:128;not null;uniqueIndex:uniq_tag_per_owner,priority:2"`
}

// TagLink is a polymorphic join from an entity (ParentType, ParentID) to a Tag.
// Uniqueness is (OwnerID, TagID, ParentType, ParentID) so a given tag can be
// assigned only once to that specific entity.
type TagLink struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	// All NOT NULL and all part of the unique constraint
	OwnerID    uint       `gorm:"not null;uniqueIndex:uniq_tag_parent,priority:1"`
	TagID      uint       `gorm:"not null;index:idx_taglink_tag;uniqueIndex:uniq_tag_parent,priority:2"`
	ParentType ParentType `gorm:"size:32;not null;index:idx_taglink_parent,priority:1;uniqueIndex:uniq_tag_parent,priority:3"`
	ParentID   uint       `gorm:"not null;index:idx_taglink_parent,priority:2;uniqueIndex:uniq_tag_parent,priority:4"`

	Tag Tag `gorm:"constraint:OnDelete:CASCADE;"`
}

func (Tag) TableName() string     { return "tags" }
func (TagLink) TableName() string { return "tag_links" }

// normalizeTag turns a user-facing name into its canonical Norm string.
func normalizeTag(s string) string {
	s = strings.TrimSpace(s)
	// Lower-casing is a simple, cross-DB approach to case-insensitive uniqueness.
	return strings.ToLower(s)
}

/*
ensureTags returns Tag rows for the given names, creating missing ones.
It is owner-scoped and safe under concurrent calls (uses INSERT ... DO NOTHING).
*/
func (crmdb *CRMDatabase) ensureTags(tx *gorm.DB, ownerID uint, names []string) ([]Tag, error) {
	if len(names) == 0 {
		return nil, nil
	}

	// Build desired set
	type wanted struct {
		name string
		norm string
	}
	var want []wanted
	seen := map[string]bool{}
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		norm := normalizeTag(n)
		if norm == "" || seen[norm] {
			continue
		}
		seen[norm] = true
		want = append(want, wanted{name: n, norm: norm})
	}
	if len(want) == 0 {
		return nil, nil
	}

	// Upsert-like: insert all (OwnerID, Name, Norm), ignore conflicts on (OwnerID, Norm)
	var inserts []Tag
	for _, w := range want {
		inserts = append(inserts, Tag{
			OwnerID: ownerID,
			Name:    w.name,
			Norm:    w.norm,
		})
	}
	if err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "owner_id"}, {Name: "norm"}},
		DoNothing: true,
	}).Create(&inserts).Error; err != nil {
		return nil, err
	}

	// Read back all tags for the owner where norm IN (...)
	var norms []string
	for _, w := range want {
		norms = append(norms, w.norm)
	}
	var tags []Tag
	if err := tx.Where("owner_id = ? AND norm IN ?", ownerID, norms).
		Find(&tags).Error; err != nil {
		return nil, err
	}
	return tags, nil
}

/*
addTagsToParent creates TagLink rows for the given parent.
It ignores duplicates thanks to the unique index + DoNothing.
*/
// comments in English
func (crmdb *CRMDatabase) addTagsToParent(tx *gorm.DB, ownerID uint, parentType ParentType, parentID uint, tags []Tag) error {
	if len(tags) == 0 {
		return nil
	}
	links := make([]TagLink, 0, len(tags))
	for _, t := range tags {
		if t.ID == 0 {
			continue
		}
		links = append(links, TagLink{
			OwnerID:    ownerID,
			TagID:      t.ID,
			ParentType: parentType,
			ParentID:   parentID,
		})
	}
	// If a soft-deleted row exists, revive it instead of doing nothing.
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "owner_id"},
			{Name: "tag_id"},
			{Name: "parent_type"},
			{Name: "parent_id"},
		},
		DoUpdates: clause.Assignments(map[string]any{
			"deleted_at": gorm.Expr("NULL"),
			"updated_at": gorm.Expr("NOW()"),
		}),
	}).Create(&links).Error
}

// Replace all links by hard-deleting previous ones, then inserting the new set,
// and finally deleting now-unused tags (only those that were removed).
func (crmdb *CRMDatabase) replaceTagsForParent(
	tx *gorm.DB,
	ownerID uint,
	parentType ParentType,
	parentID uint,
	newTags []Tag,
) error {
	// 0) Load current tag IDs for this parent (to compute removals)
	var currentIDs []uint
	if err := tx.
		Model(&TagLink{}).
		Where("owner_id = ? AND parent_type = ? AND parent_id = ?", ownerID, parentType, parentID).
		Pluck("tag_id", &currentIDs).Error; err != nil {
		return err
	}

	// 1) Hard-delete all existing links for this parent
	if err := tx.Unscoped().
		Where("owner_id = ? AND parent_type = ? AND parent_id = ?", ownerID, parentType, parentID).
		Delete(&TagLink{}).Error; err != nil {
		return err
	}

	// 2) Insert the new set (idempotent)
	if len(newTags) > 0 {
		links := make([]TagLink, 0, len(newTags))
		newIDs := make([]uint, 0, len(newTags))
		for _, t := range newTags {
			if t.ID == 0 {
				continue
			}
			newIDs = append(newIDs, t.ID)
			links = append(links, TagLink{
				OwnerID:    ownerID,
				TagID:      t.ID,
				ParentType: parentType,
				ParentID:   parentID,
			})
		}
		if len(links) > 0 {
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{
					{Name: "owner_id"},
					{Name: "tag_id"},
					{Name: "parent_type"},
					{Name: "parent_id"},
				},
				DoNothing: true,
			}).Create(&links).Error; err != nil {
				return err
			}
		}

		// 3) Compute removed IDs = current - new
		removed := diffUint(currentIDs, newIDs)

		// 4) Clean up only those removed tags that are now unused
		if err := crmdb.DeleteUnusedTagsByIDs(tx, ownerID, removed); err != nil {
			return err
		}
		return nil
	}

	// If new set is empty: everything was removed; clean up all previously attached tags
	if err := crmdb.DeleteUnusedTagsByIDs(tx, ownerID, currentIDs); err != nil {
		return err
	}
	return nil
}

// diffUint returns elements in a that are not in b.
func diffUint(a, b []uint) []uint {
	if len(a) == 0 {
		return nil
	}
	m := make(map[uint]struct{}, len(b))
	for _, x := range b {
		m[x] = struct{}{}
	}
	var out []uint
	for _, x := range a {
		if _, ok := m[x]; !ok {
			out = append(out, x)
		}
	}
	return out
}

// ListTagsForParent returns the Tag list for a given parent.
func (crmdb *CRMDatabase) ListTagsForParent(ownerID uint, parentType ParentType, parentID uint) ([]Tag, error) {
	var tags []Tag
	err := crmdb.db.
		Table("tag_links AS tl").
		Select("t.*").
		Joins("JOIN tags AS t ON t.id = tl.tag_id").
		Where("tl.owner_id = ? AND tl.parent_type = ? AND tl.parent_id = ?", ownerID, parentType, parentID).
		Where("tl.deleted_at IS NULL").
		Order("t.name ASC").
		Scan(&tags).Error
	return tags, err
}

/*
FilterCompaniesByAnyTag returns companies that have at least one of the given tag names.
Case-insensitive via Norm matching.
*/
func (crmdb *CRMDatabase) FilterCompaniesByAnyTag(ownerID uint, tagNames []string) ([]Company, error) {
	if len(tagNames) == 0 {
		return nil, nil
	}
	norms := make([]string, 0, len(tagNames))
	for _, n := range tagNames {
		n = strings.TrimSpace(n)
		if n != "" {
			norms = append(norms, normalizeTag(n))
		}
	}
	var out []Company
	err := crmdb.db.
		Table("companies c").
		Joins("JOIN tag_links tl ON tl.parent_type = ? AND tl.parent_id = c.id", ParentTypeCompany).
		Joins("JOIN tags t ON t.id = tl.tag_id").
		Where("tl.owner_id = ? AND t.norm IN ?", ownerID, norms).
		Group("c.id").
		Find(&out).Error
	return out, err
}

/*
FilterPersonsByAnyTag returns persons that have at least one of the given tag names.
*/
func (crmdb *CRMDatabase) FilterPersonsByAnyTag(ownerID uint, tagNames []string) ([]Person, error) {
	if len(tagNames) == 0 {
		return nil, nil
	}
	norms := make([]string, 0, len(tagNames))
	for _, n := range tagNames {
		n = strings.TrimSpace(n)
		if n != "" {
			norms = append(norms, normalizeTag(n))
		}
	}
	var out []Person
	err := crmdb.db.
		Table("people p").
		Joins("JOIN tag_links tl ON tl.parent_type = ? AND tl.parent_id = p.id", ParentTypePerson).
		Joins("JOIN tags t ON t.id = tl.tag_id").
		Where("tl.owner_id = ? AND t.norm IN ?", ownerID, norms).
		Group("p.id").
		Find(&out).Error
	return out, err
}

/*
Public helpers to add/replace tags by names (transactional).
*/
func (crmdb *CRMDatabase) AddTagsToCompanyByName(companyID, ownerID uint, names []string) error {
	return crmdb.db.Transaction(func(tx *gorm.DB) error {
		tags, err := crmdb.ensureTags(tx, ownerID, names)
		if err != nil {
			return err
		}
		return crmdb.addTagsToParent(tx, ownerID, ParentTypeCompany, companyID, tags)
	})
}

func (crmdb *CRMDatabase) ReplaceCompanyTagsByName(companyID, ownerID uint, names []string) error {
	return crmdb.db.Transaction(func(tx *gorm.DB) error {
		tags, err := crmdb.ensureTags(tx, ownerID, names)
		if err != nil {
			return err
		}
		return crmdb.replaceTagsForParent(tx, ownerID, ParentTypeCompany, companyID, tags)
	})
}

func (crmdb *CRMDatabase) AddTagsToPersonByName(personID, ownerID uint, names []string) error {
	return crmdb.db.Transaction(func(tx *gorm.DB) error {
		tags, err := crmdb.ensureTags(tx, ownerID, names)
		if err != nil {
			return err
		}
		return crmdb.addTagsToParent(tx, ownerID, ParentTypePerson, personID, tags)
	})
}

func (crmdb *CRMDatabase) ReplacePersonTagsByName(personID, ownerID uint, names []string) error {
	return crmdb.db.Transaction(func(tx *gorm.DB) error {
		tags, err := crmdb.ensureTags(tx, ownerID, names)
		if err != nil {
			return err
		}
		return crmdb.replaceTagsForParent(tx, ownerID, ParentTypePerson, personID, tags)
	})
}

// SuggestTags returns tags for an owner whose normalized form starts with the given prefix.
// It filters out soft-deleted rows and orders by display name (Name) ascending.
// If limit <= 0, a sensible default is used.
func (crmdb *CRMDatabase) SuggestTags(ownerID uint, prefix string, limit int) ([]Tag, error) {
	// normalize prefix the same way you normalize tags
	prefix = strings.ToLower(strings.TrimSpace(prefix))
	if prefix == "" {
		return []Tag{}, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 10
	}

	var out []Tag
	err := crmdb.db.
		Where("owner_id = ? AND norm LIKE ?", ownerID, prefix+"%").
		Order("name ASC").
		Limit(limit).
		Find(&out).Error
	return out, err
}

// SuggestTagNames is a convenience that returns only the display names.
func (crmdb *CRMDatabase) SuggestTagNames(ownerID uint, prefix string, limit int) ([]string, error) {
	tags, err := crmdb.SuggestTags(ownerID, prefix, limit)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(tags))
	for _, t := range tags {
		names = append(names, t.Name)
	}
	return names, nil
}

// DeleteUnusedTagsByIDs removes tags for this owner that are in the provided ID set
// and are no longer referenced by tag_links. Safe to call inside the same transaction.
// Hard-delete unused tags among the provided IDs (owner-scoped).
func (crmdb *CRMDatabase) DeleteUnusedTagsByIDs(tx *gorm.DB, ownerID uint, tagIDs []uint) error {
	if len(tagIDs) == 0 {
		return nil
	}
	return tx.Unscoped().
		Where("owner_id = ? AND id IN ?", ownerID, tagIDs).
		Where("NOT EXISTS (?)",
			tx.Table("tag_links").Select("1").Where("tag_links.tag_id = tags.id"),
		).
		Delete(&Tag{}).Error
}
