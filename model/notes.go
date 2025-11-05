package model

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// Note represents a user-authored text entry attached to a parent entity
// (either a person or a company). Notes are owner-scoped and can be tagged.
//
// ParentType determines the kind of parent ("people" or "companies").
// The combination (OwnerID, ParentType, ParentID) defines the attachment target.
//
// Notes are lightweight, versionless records. EditedAt is automatically updated
// on save to reflect the last modification time.
type Note struct {
	gorm.Model
	OwnerID    uint      `json:"owner_id"    form:"owner_id"`                 // Set server-side: tenant/owner scope
	AuthorID   uint      `json:"author_id"   form:"-"           gorm:"index"` // Set server-side: creating user
	ParentID   uint      `json:"parent_id"   form:"parent_id"`                // ID of the parent record
	ParentType string    `json:"parent_type" form:"parent_type"`              // "people" | "companies"
	Title      string    `json:"title"       form:"title"`                    // Optional headline
	Body       string    `json:"body"        form:"body"`                     // Main text content
	Tags       string    `json:"tags"        form:"tags"`                     // Comma-separated tags (stored as CSV)
	EditedAt   time.Time `json:"edited_at"   form:"edited_at"`                // Usually managed server-side
}

// BeforeSave GORM hook — automatically updates EditedAt timestamp
// whenever the record is saved.
func (n *Note) BeforeSave(tx *gorm.DB) error {
	n.EditedAt = time.Now()
	return nil
}

// -----------------------
// Helper functions
// -----------------------

// checkParentType checks that the given parent type is valid
// and returns the normalized value.
//
// Valid parent types are:
//   - "people"
//   - "companies"
func checkParentType(s string) (string, error) {
	switch s {
	case "people", "companies":
		return s, nil
	default:
		return "", fmt.Errorf("invalid parent_type %q (expected 'people' or 'companies')", s)
	}
}

// SplitTags splits a comma-separated string into a cleaned slice of tags,
// trimming whitespace and skipping empty entries.
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

// JoinTags joins a slice of tag strings into a single comma-separated value,
// trimming extra spaces.
func JoinTags(a []string) string {
	for i := range a {
		a[i] = strings.TrimSpace(a[i])
	}
	return strings.Join(a, ",")
}

// -----------------------
// Database methods
// -----------------------

// CreateNote inserts a new note record after normalizing its ParentType.
// EditedAt is automatically set via BeforeSave.
func (crmdb *CRMDatabase) CreateNote(n *Note) error {
	pt, err := checkParentType(n.ParentType)
	if err != nil {
		return err
	}
	n.ParentType = pt
	return crmdb.db.Create(n).Error
}

// GetNoteByID loads a single note by ID, ensuring it belongs to the given owner.
func (crmdb *CRMDatabase) GetNoteByID(id uint, ownerID uint) (*Note, error) {
	var n Note
	err := crmdb.db.
		Where("id = ? AND owner_id = ?", id, ownerID).
		First(&n).Error
	if err != nil {
		return nil, err
	}
	return &n, nil
}

// NoteFilters provides filtering and paging parameters for listing notes.
type NoteFilters struct {
	Search     string // Optional: search query (matches title/body/tags)
	Limit      int    // Page size (defaults to 50; capped at 200)
	Offset     int    // Offset for pagination
	ParentType string // Optional: filter by parent type
	ParentID   uint   // Optional: filter by parent ID
}

// LoadAllNotesForParent is a convenience wrapper around ListNotesForParent
// using default (empty) filters.
func (crmdb *CRMDatabase) LoadAllNotesForParent(ownerID uint, parentType string, parentID uint) ([]Note, error) {
	return crmdb.ListNotesForParent(ownerID, parentType, parentID, NoteFilters{})
}

// ListNotesForParent returns a list of notes belonging to a given parent entity,
// optionally filtered by search terms, with pagination support.
//
// Search applies a simple LIKE filter over title, body, and tags (case-sensitive by default).
func (crmdb *CRMDatabase) ListNotesForParent(ownerID uint, parentType string, parentID uint, f NoteFilters) ([]Note, error) {
	pt, err := checkParentType(parentType)
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

// UpdateNoteContentAsAuthor allows the author of a note to update its content.
// Enforces that the current author matches the note's AuthorID.
//
// Only title, body, tags, and edited_at are updated.
func (crmdb *CRMDatabase) UpdateNoteContentAsAuthor(ownerID, authorID, noteID uint, title, body, tags string) (*Note, error) {
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

// DeleteNote removes a note by ID, restricted to its owner and author.
// Authors can only delete their own notes.
func (crmdb *CRMDatabase) DeleteNote(id uint, ownerID uint, authorID uint) error {
	return crmdb.db.
		Where("id = ? AND owner_id = ? AND author_id = ?", id, ownerID, authorID).
		Delete(&Note{}).Error
}
