package model

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

type FieldKind string

const (
	// Fixed set for the editor
	FieldSender      FieldKind = "addressee"    // "Recipient"
	FieldInvoiceInfo FieldKind = "invoice_info" // "Rechnungsangaben"
	FieldPositions   FieldKind = "main_area"    // table area (may have page 2 coords)
)

// LetterheadTemplate represents a letterhead (1–2 pages) with optional predefined regions.
type LetterheadTemplate struct {
	gorm.Model
	OwnerID         uint    `gorm:"index"`
	Name            string  `gorm:"size:200"`
	PageWidthCm     float64 // e.g., 21.0 (A4)
	PageHeightCm    float64 // e.g., 29.7 (A4)
	PDFPath         string  // server path to original PDF (optional)
	PreviewPage1URL string  // public URL to PNG page 1
	PreviewPage2URL string  // public URL to PNG page 2 (optional)
	// Important: explicit foreignKey mapping so GORM understands TemplateID below.
	Regions []PlacedRegion `gorm:"foreignKey:TemplateID;references:ID;constraint:OnDelete:CASCADE"`

	FontNormal string `gorm:"size:255"` // e.g. "Inter-Regular.ttf"
	FontBold   string `gorm:"size:255"` // e.g. "Inter-Bold.ttf"
	FontItalic string `gorm:"size:255"` // e.g. "Inter-Italic.ttf"
}

// PlacedRegion stores a draggable/resizable region (positions in cm).
// For kind == "main_area", the optional second-page rectangle is controlled via HasPage2 + *2 fields.
type PlacedRegion struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`

	TemplateID uint      `gorm:"index:idx_regions_tpl_owner;uniqueIndex:uniq_tpl_owner_kind" json:"template_id"` // FK -> LetterheadTemplate.ID
	OwnerID    uint      `gorm:"index:idx_regions_tpl_owner;uniqueIndex:uniq_tpl_owner_kind" json:"owner_id"`
	Kind       FieldKind `gorm:"type:text;uniqueIndex:uniq_tpl_owner_kind" json:"kind"`

	// Primary rectangle (typically page 1)
	Page     int     `json:"page"` // kept for compatibility; typically 1
	XCm      float64 `json:"xCm"`
	YCm      float64 `json:"yCm"`
	WidthCm  float64 `json:"widthCm"`
	HeightCm float64 `json:"heightCm"`

	// Text/layout options
	HAlign      string  `gorm:"size:10" json:"hAlign"`   // left|center|right
	VAlign      string  `gorm:"size:10" json:"vAlign"`   // top|middle|bottom (optional)
	FontName    string  `gorm:"size:50" json:"fontName"` // e.g., Helvetica
	FontSizePt  float64 `json:"fontSizePt"`
	LineSpacing float64 `json:"lineSpacing"`

	// Optional second-page rectangle for kind == "main_area"
	HasPage2  bool    `gorm:"not null;default:false" json:"hasPage2"`
	X2Cm      float64 `json:"x2Cm"`
	Y2Cm      float64 `json:"y2Cm"`
	Width2Cm  float64 `json:"width2Cm"`
	Height2Cm float64 `json:"height2Cm"`
}

func (PlacedRegion) TableName() string { return "letterhead_regions" }

type TemplateFonts struct {
	Normal string
	Bold   string
	Italic string
}

// -------------------- Data access on CRMDatabase (no DB usage in controllers) --------------------

// SaveLetterheadTemplate creates or updates a letterhead template. Ownership is enforced.
func (db *CRMDatabase) SaveLetterheadTemplate(t *LetterheadTemplate, ownerID uint) error {
	if t.OwnerID != ownerID {
		return errors.New("save letterhead: ownerid mismatch")
	}
	return db.db.Transaction(func(tx *gorm.DB) error {
		return tx.Save(t).Error
	})
}

// LoadLetterheadTemplate loads a template (including its regions) for a given owner.
func (db *CRMDatabase) LoadLetterheadTemplate(id, ownerID uint) (*LetterheadTemplate, error) {
	var t LetterheadTemplate
	if err := db.db.Preload("Regions").
		Where("id = ? AND owner_id = ?", id, ownerID).
		First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

// ListLetterheadTemplates returns all templates for a given owner.
func (db *CRMDatabase) ListLetterheadTemplates(ownerID uint) ([]LetterheadTemplate, error) {
	var list []LetterheadTemplate
	if err := db.db.Where("owner_id = ?", ownerID).
		Order("updated_at DESC").
		Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (db *CRMDatabase) LoadLetterheadTemplateAnyOwner(id uint) (*LetterheadTemplate, error) {
	var t LetterheadTemplate
	if err := db.db.Preload("Regions").
		Where("id = ?", id).
		First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

// Optional Convenience: kapselt die Zugriffspolitik
func (db *CRMDatabase) LoadLetterheadTemplateForAccess(id, ownerID uint, isAdmin bool) (*LetterheadTemplate, error) {
	if isAdmin {
		return db.LoadLetterheadTemplateAnyOwner(id)
	}
	return db.LoadLetterheadTemplate(id, ownerID)
}

// UpdateLetterheadPageSize updates page size (in cm) of a template.
func (db *CRMDatabase) UpdateLetterheadPageSize(id, ownerID uint, wcm, hcm float64) error {
	return db.db.Model(&LetterheadTemplate{}).
		Where("id = ? AND owner_id = ?", id, ownerID).
		Updates(map[string]any{
			"page_width_cm":  wcm,
			"page_height_cm": hcm,
		}).Error
}

// UpdateLetterheadPreviewURLs updates preview image URLs.
func (db *CRMDatabase) UpdateLetterheadPreviewURLs(id, ownerID uint, page1URL, page2URL string) error {
	return db.db.Model(&LetterheadTemplate{}).
		Where("id = ? AND owner_id = ?", id, ownerID).
		Updates(map[string]any{
			"preview_page1_url": page1URL,
			"preview_page2_url": page2URL,
		}).Error
}

// EnsureDefaultLetterheadRegions makes sure the three fixed regions exist for the template.
// It creates missing ones with sane defaults, but does not delete anything.
func (db *CRMDatabase) EnsureDefaultLetterheadRegions(templateID, ownerID uint, pageWidthCm, pageHeightCm float64) error {
	return db.db.Transaction(func(tx *gorm.DB) error {
		var existing []PlacedRegion
		if err := tx.Where("template_id = ? AND owner_id = ?", templateID, ownerID).
			Find(&existing).Error; err != nil {
			return err
		}
		seen := map[FieldKind]bool{}
		for _, r := range existing {
			seen[r.Kind] = true
		}
		var toCreate []PlacedRegion
		if !seen[FieldSender] {
			toCreate = append(toCreate, PlacedRegion{
				TemplateID: templateID, OwnerID: ownerID, Kind: FieldSender,
				Page: 1, XCm: 2, YCm: 2, WidthCm: 8, HeightCm: 3,
				HAlign: "left", FontSizePt: 10, LineSpacing: 1.2,
			})
		}
		if !seen[FieldInvoiceInfo] {
			toCreate = append(toCreate, PlacedRegion{
				TemplateID: templateID, OwnerID: ownerID, Kind: FieldInvoiceInfo,
				Page: 1, XCm: pageWidthCm - 8.0 - 2.0, YCm: 2, WidthCm: 8.0, HeightCm: 4.0,
				HAlign: "right", FontSizePt: 10, LineSpacing: 1.2,
			})
		}
		if !seen[FieldPositions] {
			toCreate = append(toCreate, PlacedRegion{
				TemplateID: templateID, OwnerID: ownerID, Kind: FieldPositions,
				Page: 1, XCm: 2.0, YCm: 6.0, WidthCm: pageWidthCm - 4.0, HeightCm: pageHeightCm - 8.0,
				HAlign: "left", FontSizePt: 10, LineSpacing: 1.2,
				HasPage2: false,
			})
		}
		if len(toCreate) > 0 {
			if err := tx.Create(&toCreate).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// UpdateLetterheadRegionsAndFonts speichert Regions und zusätzlich
// Template-Meta (Fonts + Page-Size) atomar in einer Transaktion.
func (db *CRMDatabase) UpdateLetterheadRegionsAndFonts(
	templateID, ownerID uint,
	regions []PlacedRegion,
	fonts *TemplateFonts,
	pageW, pageH float64,
) error {
	allowed := map[FieldKind]bool{
		FieldSender: true, FieldInvoiceInfo: true, FieldPositions: true,
	}

	return db.db.Transaction(func(tx *gorm.DB) error {
		var tpl LetterheadTemplate
		if err := tx.Select("id, owner_id").
			Where("id = ? AND owner_id = ?", templateID, ownerID).
			First(&tpl).Error; err != nil {
			return err
		}

		meta := map[string]any{}
		if pageW > 0 {
			meta["page_width_cm"] = pageW
		}
		if pageH > 0 {
			meta["page_height_cm"] = pageH
		}
		if fonts != nil {
			meta["font_normal"] = fonts.Normal
			meta["font_bold"] = fonts.Bold
			meta["font_italic"] = fonts.Italic
		}
		if len(meta) > 0 {
			if err := tx.Model(&LetterheadTemplate{}).
				Where("id = ? AND owner_id = ?", templateID, ownerID).
				Updates(meta).Error; err != nil {
				return err
			}
		}

		var current []PlacedRegion
		if err := tx.Where("template_id = ? AND owner_id = ?", templateID, ownerID).
			Find(&current).Error; err != nil {
			return err
		}
		curByKind := map[FieldKind]*PlacedRegion{}
		for i := range current {
			r := &current[i]
			curByKind[r.Kind] = r
		}

		for _, in := range regions {
			if !allowed[in.Kind] {
				continue
			}
			if ex, ok := curByKind[in.Kind]; ok {
				// Update allowed fields
				ex.Page = 1
				ex.XCm, ex.YCm, ex.WidthCm, ex.HeightCm = in.XCm, in.YCm, in.WidthCm, in.HeightCm
				ex.HAlign, ex.VAlign = in.HAlign, in.VAlign
				ex.FontName, ex.FontSizePt, ex.LineSpacing = in.FontName, in.FontSizePt, in.LineSpacing
				if in.Kind == FieldPositions {
					ex.HasPage2 = in.HasPage2
					ex.X2Cm, ex.Y2Cm, ex.Width2Cm, ex.Height2Cm = in.X2Cm, in.Y2Cm, in.Width2Cm, in.Height2Cm
				}
				if err := tx.Save(ex).Error; err != nil {
					return err
				}
			} else {
				// Create missing fixed region (legacy)
				in.ID = 0
				in.TemplateID = templateID
				in.OwnerID = ownerID
				if in.Kind != FieldPositions {
					in.HasPage2, in.X2Cm, in.Y2Cm, in.Width2Cm, in.Height2Cm = false, 0, 0, 0, 0
				}
				if in.Page <= 0 {
					in.Page = 1
				}
				if in.HAlign == "" {
					in.HAlign = "left"
				}
				if in.FontSizePt == 0 {
					in.FontSizePt = 10
				}
				if in.LineSpacing == 0 {
					in.LineSpacing = 1.2
				}
				if err := tx.Create(&in).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// DeleteLetterheadTemplate deletes a template (regions auto-delete via CASCADE).
func (db *CRMDatabase) DeleteLetterheadTemplate(id, ownerID uint) error {
	return db.db.Where("id = ? AND owner_id = ?", id, ownerID).
		Delete(&LetterheadTemplate{}).Error
}
