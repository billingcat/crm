package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// -----------------------
// GORM Model
// -----------------------

type Note struct {
	gorm.Model
	OwnerID    uint      `json:"owner_id"    form:"owner_id"`                 // vom Server gesetzt
	AuthorID   uint      `json:"author_id"   form:"-"           gorm:"index"` // vom Server gesetzt
	ParentID   uint      `json:"parent_id"   form:"parent_id"`
	ParentType string    `json:"parent_type" form:"parent_type"` // "people" | "companies"
	Title      string    `json:"title"       form:"title"`
	Body       string    `json:"body"        form:"body"`
	Tags       string    `json:"tags"        form:"tags"`      // CSV
	EditedAt   time.Time `json:"edited_at"   form:"edited_at"` // i. d. R. serverseitig setzen
}

func (n *Note) BeforeSave(tx *gorm.DB) error {
	n.EditedAt = time.Now()
	return nil
}

// -----------------------
// Helper functions
// -----------------------

func normalizeParentType(s string) (string, error) {
	switch strings.ToLower(s) {
	case "person", "people", "persons":
		return "people", nil
	case "company", "companies", "firma", "firmen":
		return "companies", nil
	default:
		return "", errors.New("unsupported parent_type (use 'people' or 'companies')")
	}
}

func SplitTags(s string) []string {
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func JoinTags(a []string) string {
	for i := range a {
		a[i] = strings.TrimSpace(a[i])
	}
	return strings.Join(a, ",")
}

// -----------------------
// database methods
// -----------------------

func (crmdb *CRMDatenbank) AutoMigrateNotes() error {
	return crmdb.db.AutoMigrate(&Note{})
}

func (crmdb *CRMDatenbank) CreateNote(n *Note) error {
	pt, err := normalizeParentType(n.ParentType)
	if err != nil {
		return err
	}
	n.ParentType = pt
	return crmdb.db.Create(n).Error
}

func (crmdb *CRMDatenbank) GetNoteByID(id uint, ownerID uint) (*Note, error) {
	var n Note
	err := crmdb.db.
		Where("id = ? AND owner_id = ?", id, ownerID).
		First(&n).Error
	if err != nil {
		return nil, err
	}
	return &n, nil
}

type NoteFilters struct {
	Search     string
	Limit      int
	Offset     int
	ParentType string
	ParentID   uint
}

func (crmdb *CRMDatenbank) LoadAllNotesForParent(ownerID uint, parentType string, parentID uint) ([]Note, error) {
	return crmdb.ListNotesForParent(ownerID, parentType, parentID, NoteFilters{})
}

func (crmdb *CRMDatenbank) ListNotesForParent(ownerID uint, parentType string, parentID uint, f NoteFilters) ([]Note, error) {
	pt, err := normalizeParentType(parentType)
	if err != nil {
		return nil, err
	}

	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := f.Offset

	q := crmdb.db.
		Where("owner_id = ? AND parent_type = ? AND parent_id = ?", ownerID, pt, parentID)

	if s := strings.TrimSpace(f.Search); s != "" {
		like := "%" + s + "%"
		q = q.Where("title LIKE ? OR body LIKE ? OR tags LIKE ?", like, like, like)
	}

	var notes []Note
	err = q.
		Order("edited_at DESC, id DESC").
		Limit(limit).
		Offset(offset).
		Find(&notes).Error
	return notes, err
}

func (crmdb *CRMDatenbank) UpdateNoteContentAsAuthor(ownerID, authorID, noteID uint, title, body, tags string) (*Note, error) {
	var n Note
	if err := crmdb.db.Where("id = ? AND owner_id = ?", noteID, ownerID).First(&n).Error; err != nil {
		return nil, err
	}
	if n.AuthorID != authorID {
		return nil, fmt.Errorf("forbidden")
	}
	n.Title = title
	n.Body = body
	n.Tags = tags
	n.EditedAt = time.Now()

	if err := crmdb.db.Model(&n).Select("Title", "Body", "Tags", "EditedAt").Updates(&n).Error; err != nil {
		return nil, err
	}
	return &n, nil
}

func (crmdb *CRMDatenbank) DeleteNote(id uint, ownerID uint, authorID uint) error {
	return crmdb.db.
		Where("id = ? AND owner_id = ? AND author_id = ?", id, ownerID, authorID).
		Delete(&Note{}).Error
}
