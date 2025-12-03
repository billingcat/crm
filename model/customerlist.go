package model

import (
	"fmt"
	"strings"
	"unicode"
)

type TagCount struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

// ListOwnerCompanyTags returns all tag names used on companies for a given owner with usage counts.
// Soft-deleted links are ignored.
func (s *Store) ListOwnerCompanyTags(ownerID uint) ([]TagCount, error) {
	var rows []TagCount
	// Only company links
	err := s.db.
		Table("tag_links tl").
		Select("t.name AS name, COUNT(*) AS count").
		Joins("JOIN tags t ON t.id = tl.tag_id").
		Where("tl.owner_id = ? AND tl.parent_type = ? AND tl.deleted_at IS NULL", ownerID, ParentTypeCompany).
		Group("t.name").
		Order("LOWER(t.name) ASC").
		Scan(&rows).Error
	return rows, err
}

// CompanyListFilters is the input for the company search.
type CompanyListFilters struct {
	Query   string   // optional free text
	Tags    []string // display names from UI (we normalize internally)
	ModeAND bool     // true: entity must have ALL tags; false: ANY of tags
	Limit   int
	Offset  int
}

// CompanyListResult bundles page results.
type CompanyListResult struct {
	Companies []Company
	Total     int64
}

// SearchCompaniesByTags performs a filtered search with pagination.
// Notes:
// - Tag names are normalized via normalizeTagName() -> match via tags.norm
// - ModeAND is handled by a HAVING count(distinct tag_id) = len(tags)
// - Query matches company name and (optional) email/domain fields if you have them
func (s *Store) SearchCompaniesByTags(ownerID uint, f CompanyListFilters) (CompanyListResult, error) {
	if f.Limit <= 0 {
		f.Limit = 25
	}
	if f.Limit > 200 {
		f.Limit = 200
	}
	if f.Offset < 0 {
		f.Offset = 0
	}

	// Base scope: owner companies
	base := s.db.Model(&Company{}).Where("owner_id = ?", ownerID)

	// Free-text query (expand fields as you like)
	if q := strings.TrimSpace(f.Query); q != "" {
		// list of searchable columns
		searchCols := []string{"name", "land", "ort", "kundennummer"}

		if s.db.Dialector.Name() == "postgres" {
			p := "%" + q + "%"
			var ors []string
			for range searchCols {
				ors = append(ors, "?? ILIKE ?")
			}
			// join to "(col1 ILIKE ? OR col2 ILIKE ? ...)"
			where := "(" + strings.Join(ors, " OR ") + ")"

			// replace ?? with column names and build args
			args := make([]any, 0, len(searchCols))
			for _, col := range searchCols {
				where = strings.Replace(where, "??", col, 1)
				args = append(args, p)
			}
			base = base.Where(where, args...)
		} else {
			p := "%" + strings.ToLower(q) + "%"
			var ors []string
			for _, col := range searchCols {
				ors = append(ors, fmt.Sprintf("LOWER(%s) LIKE ?", col))
			}
			where := "(" + strings.Join(ors, " OR ") + ")"
			args := make([]any, len(searchCols))
			for i := range searchCols {
				args[i] = p
			}
			base = base.Where(where, args...)
		}
	}
	// Tag filtering?
	norms := make([]string, 0, len(f.Tags))
	for _, name := range f.Tags {
		if n := normalizeTagName(name); n != "" {
			norms = append(norms, n)
		}
	}

	var result CompanyListResult

	if len(norms) == 0 {
		// No tag filter → simple count + page
		if err := base.Count(&result.Total).Error; err != nil {
			return result, err
		}
		var rows []Company
		if err := base.
			Preload("ContactInfos", "parent_type = ? AND deleted_at IS NULL", ParentTypeCompany).
			Order("LOWER(name) ASC, id ASC").
			Limit(f.Limit).Offset(f.Offset).
			Find(&rows).Error; err != nil {
			return result, err
		}
		result.Companies = rows
		return result, nil
	}

	// With tag filter: build a subquery joining tag_links/tags and grouping by company_id.
	// Filter only company links and owner scope, and only desired tag norms.
	linkSub := s.db.
		Table("tag_links tl").
		Select("tl.parent_id AS company_id, COUNT(DISTINCT tl.tag_id) AS hit_count").
		Joins("JOIN tags t ON t.id = tl.tag_id").
		Where("tl.owner_id = ? AND tl.parent_type = ? AND tl.deleted_at IS NULL", ownerID, ParentTypeCompany).
		Where("t.norm IN ?", norms).
		Group("tl.parent_id")

	// For AND, required hits == number of norms; for OR, >= 1.
	required := 1
	if f.ModeAND {
		required = len(norms)
	}
	linkSub = linkSub.Having("COUNT(DISTINCT tl.tag_id) >= ?", required)

	// Join subquery to company base
	withTags := base.Joins("JOIN (?) tagf ON tagf.company_id = companies.id", linkSub)

	// Count
	if err := withTags.Count(&result.Total).Error; err != nil {
		return result, err
	}

	// Page
	var rows []Company
	if err := withTags.
		Preload("ContactInfos", "parent_type = ? AND deleted_at IS NULL", ParentTypeCompany).
		Order("LOWER(companies.name) ASC, companies.id ASC").
		Limit(f.Limit).Offset(f.Offset).
		Find(&rows).Error; err != nil {
		return result, err
	}
	result.Companies = rows
	return result, nil
}

// Helper for building a canonical pagination URL (optional)
func BuildCustomerListURL(basePath string, q string, tags []string, modeAND bool, page, pageSize int) string {
	var b strings.Builder
	b.WriteString(basePath)
	sep := "?"
	if q != "" {
		b.WriteString(sep + "q=" + urlQueryEscape(q))
		sep = "&"
	}
	for _, t := range tags {
		b.WriteString(fmt.Sprintf("%stags=%s", sep, urlQueryEscape(t)))
		sep = "&"
	}
	if modeAND {
		b.WriteString(sep + "mode=and")
		sep = "&"
	}
	if page > 1 {
		b.WriteString(fmt.Sprintf("%sp=%d", sep, page))
		sep = "&"
	}
	if pageSize > 0 && pageSize != 25 {
		b.WriteString(fmt.Sprintf("%sps=%d", sep, pageSize))
	}
	return b.String()
}

// Minimal URL escape util to avoid importing net/url everywhere in templates
func urlQueryEscape(s string) string {
	repl := strings.NewReplacer(" ", "+")
	return repl.Replace(s) // simplistic; good enough for tags/names; use url.QueryEscape if you prefer
}

// normalizeTagName produces a DB-agnostic, case-insensitive canonical form
// used for the 'Norm' field of Tag. It ensures that tags like "VIP",
// "vip", or "  vIp  " map to the same normalized key.
//
// Behaviour:
//   - trims leading/trailing whitespace
//   - collapses internal whitespace to a single space
//   - lowercases all runes using Unicode rules (no locale dependence)
//   - returns the cleaned string; empty input -> ""
//
// Example:
//
//	normalizeTagName("  VIP  Kunden ") -> "vip kunden"
func normalizeTagName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(s))
	var prevSpace bool

	for _, r := range s {
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteRune(' ')
				prevSpace = true
			}
			continue
		}
		prevSpace = false
		b.WriteRune(unicode.ToLower(r))
	}

	return strings.TrimSpace(b.String())
}
