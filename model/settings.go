package model

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Settings holds tenant-scoped account data such as address, invoicing details,
// and bank information. We keep the embedded gorm.Model (ID + timestamps) but
// enforce a UNIQUE owner_id so there is at most one settings row per owner.
type Settings struct {
	gorm.Model
	OwnerID               uint   `gorm:"uniqueIndex;column:owner_id"` // One row per owner/tenant
	CompanyName           string `gorm:"column:company_name"`
	InvoiceContact        string `gorm:"column:invoice_contact"`
	InvoiceEMail          string `gorm:"column:invoice_email"` // stored as invoice_email (not invoice_e_mail)
	ZIP                   string `gorm:"column:zip"`
	Address1              string `gorm:"column:address1"`
	Address2              string `gorm:"column:address2"`
	City                  string `gorm:"column:city"`
	CountryCode           string `gorm:"column:country_code"` // ISO 3166-1 alpha-2 recommended
	VATID                 string `gorm:"column:vat_id"`
	TAXNumber             string `gorm:"column:tax_number"`
	InvoiceNumberTemplate string `gorm:"column:invoice_number_template"` // e.g. "INV-{YYYY}-{NNNN}"
	UseLocalCounter       bool   `gorm:"column:use_local_counter"`       // if true, number increments per owner locally
	BankIBAN              string `gorm:"column:bank_iban"`
	BankName              string `gorm:"column:bank_name"`
	BankBIC               string `gorm:"column:bank_bic"`
	CustomerNumberPrefix  string `gorm:"column:customer_number_prefix"`  // e.g. "K-"
	CustomerNumberWidth   int    `gorm:"column:customer_number_width"`   // e.g. 5 -> K-00001
	CustomerNumberCounter int64  `gorm:"column:customer_number_counter"` // current counter (e.g. 1000)
}

// LoadSettings loads the settings row for a given owner.
// Accepts ownerID as uint or int and returns an initialized (but unsaved)
// Settings record if none exists yet (via FirstOrInit).
func (s *Store) LoadSettings(ownerID any) (*Settings, error) {
	var oid uint
	switch v := ownerID.(type) {
	case uint:
		oid = v
	case int:
		oid = uint(v)
	default:
		return nil, fmt.Errorf("LoadSettings: unsupported ownerID type %T", ownerID)
	}

	settings := &Settings{}
	// FirstOrInit: if no row exists, return a struct prefilled with OwnerID
	// without hitting INSERT (use Save/SaveSettings later to persist).
	if err := s.db.
		Where("owner_id = ?", oid).
		FirstOrInit(settings, &Settings{OwnerID: oid}).Error; err != nil {
		return nil, err
	}
	return settings, nil
}

// UpdateSettings updates fields for the existing row identified by owner_id.
// Uses an explicit WHERE owner_id filter to avoid accidentally updating by the
// primary key (ID) if the struct carries a different ID value.
//
// Note: updated_at uses NOW() which is DB-specific; for SQLite you may prefer
// CURRENT_TIMESTAMP or let GORM manage timestamps automatically.
func (s *Store) UpdateSettings(settings *Settings) error {
	if settings.OwnerID == 0 {
		return errors.New("UpdateSettings: OwnerID required")
	}
	return s.db.
		Model(&Settings{}).
		Where("owner_id = ?", settings.OwnerID).
		Updates(map[string]any{
			"company_name":            settings.CompanyName,
			"invoice_contact":         settings.InvoiceContact,
			"invoice_email":           settings.InvoiceEMail,
			"zip":                     settings.ZIP,
			"address1":                settings.Address1,
			"address2":                settings.Address2,
			"city":                    settings.City,
			"country_code":            settings.CountryCode,
			"vat_id":                  settings.VATID,
			"tax_number":              settings.TAXNumber,
			"invoice_number_template": settings.InvoiceNumberTemplate,
			"use_local_counter":       settings.UseLocalCounter,
			"bank_iban":               settings.BankIBAN,
			"bank_name":               settings.BankName,
			"bank_bic":                settings.BankBIC,
			"customer_number_prefix":  settings.CustomerNumberPrefix,
			"customer_number_width":   settings.CustomerNumberWidth,
			"customer_number_counter": settings.CustomerNumberCounter,
			"updated_at":              gorm.Expr("NOW()"),
		}).Error
}

// SaveSettings performs an upsert keyed by owner_id (ON CONFLICT DO UPDATE).
// If a row for owner_id exists, the listed columns are updated; otherwise, a new
// row is inserted.
//
// Caveat: GORM translates ON CONFLICT per dialect. Ensure a unique index exists
// on owner_id (declared on the struct) and that the target DB supports the clause.
func (s *Store) SaveSettings(settings *Settings) error {
	if settings.OwnerID == 0 {
		return errors.New("SaveSettings: OwnerID required")
	}
	return s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "owner_id"}}, // conflict target
		DoUpdates: clause.Assignments(map[string]any{
			"company_name":            settings.CompanyName,
			"invoice_contact":         settings.InvoiceContact,
			"invoice_email":           settings.InvoiceEMail,
			"zip":                     settings.ZIP,
			"address1":                settings.Address1,
			"address2":                settings.Address2,
			"city":                    settings.City,
			"country_code":            settings.CountryCode,
			"vat_id":                  settings.VATID,
			"tax_number":              settings.TAXNumber,
			"invoice_number_template": settings.InvoiceNumberTemplate,
			"use_local_counter":       settings.UseLocalCounter,
			"bank_iban":               settings.BankIBAN,
			"bank_name":               settings.BankName,
			"bank_bic":                settings.BankBIC,
			"customer_number_prefix":  settings.CustomerNumberPrefix,
			"customer_number_width":   settings.CustomerNumberWidth,
			"customer_number_counter": settings.CustomerNumberCounter,

			// ensure updated_at changes on UPSERT
			"updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
		}),
	}).Create(settings).Error
}

// formatCustomerNumber builds the display string: prefix + zero-padded width + n (e.g. "K-" + 5 + 42 => "K-00042").
func formatCustomerNumber(prefix string, width int, n int64) string {
	if width < 0 {
		width = 0
	}
	return fmt.Sprintf("%s%0*d", prefix, width, n)
}

// ErrNoSettingsRow is returned when no settings row exists in the database.
var ErrNoSettingsRow = errors.New("no settings row found")

// NextCustomerNumberTx allocates the next unique customer number in a transaction.
// Returns the formatted string and the numeric value used.
func (s *Store) NextCustomerNumberTx(ctx context.Context) (string, int64, error) {
	var result string
	var numeric int64

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Lock settings row for update (Postgres/MySQL). SQLite ignores this clause.
		var s Settings
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&s).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNoSettingsRow
			}
			return err
		}

		// Try from counter+1 upwards until free.
		tryVal := s.CustomerNumberCounter + 1
		for {
			candidate := formatCustomerNumber(s.CustomerNumberPrefix, s.CustomerNumberWidth, tryVal)
			var cnt int64
			if err := tx.Model(&Company{}).
				Where("customer_number = ?", candidate).
				Count(&cnt).Error; err != nil {
				return err
			}
			if cnt == 0 {
				// Found free number -> persist counter and return
				s.CustomerNumberCounter = tryVal
				if err := tx.Model(&Settings{}).Where("id = ?", s.ID).
					Updates(map[string]any{
						"customer_number_counter": s.CustomerNumberCounter,
					}).Error; err != nil {
					return err
				}
				result = candidate
				numeric = tryVal
				return nil
			}
			tryVal++
		}
	})
	return result, numeric, err
}

// parseNumericPart extracts the numeric tail after the configured prefix.
func parseNumericPart(prefix, s string) (int64, bool) {
	if !strings.HasPrefix(s, prefix) {
		return 0, false
	}
	tail := strings.TrimPrefix(s, prefix)
	if tail == "" {
		return 0, false
	}
	for _, r := range tail {
		if !unicode.IsDigit(r) {
			return 0, false
		}
	}
	var n int64
	_, err := fmt.Sscan(tail, &n)
	if err != nil {
		return 0, false
	}
	return n, true
}

// --- Public API ---

// SuggestNextCustomerNumber returns a non-persistent suggestion (counter+1 formatted).
func (s *Store) SuggestNextCustomerNumber(ctx context.Context) (string, error) {
	var settings Settings
	err := s.db.WithContext(ctx).First(&settings).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// domain specific error when no settings row exists
			return "", ErrNoSettingsRow
		}
		// other errors
		return "", err
	}

	n := settings.CustomerNumberCounter + 1
	return formatCustomerNumber(settings.CustomerNumberPrefix, settings.CustomerNumberWidth, n), nil
}

// CheckCustomerNumber validates whether a customer number is valid and available.
//
// It enforces format rules from settings (prefix and numeric width) and checks uniqueness.
// Returns:
//
//	ok=true  -> number is syntactically valid and available (or belongs to excludeID)
//	ok=false -> invalid or taken; message gives human-readable reason
func (s *Store) CheckCustomerNumber(ctx context.Context, num string, excludeID uint) (ok bool, message string, err error) {
	// Empty -> treated as a neutral suggestion
	if num == "" {
		return true, "Vorschlag – kann überschrieben werden.", nil
	}

	// Load settings for validation rules
	var settings Settings
	if err := s.db.WithContext(ctx).First(&settings).Error; err != nil {
		return false, "Fehler beim Laden der Einstellungen", err
	}

	prefix := strings.TrimSpace(settings.CustomerNumberPrefix)
	width := settings.CustomerNumberWidth

	// Check prefix
	if prefix != "" && !strings.HasPrefix(num, prefix) {
		return false, fmt.Sprintf("Kundennummer muss mit „%s“ beginnen", prefix), nil
	}

	// Extract numeric tail after prefix
	tail := strings.TrimPrefix(num, prefix)
	if tail == "" {
		return false, "Fehlende Zahl nach Präfix", nil
	}

	for _, r := range tail {
		if !unicode.IsDigit(r) {
			return false, "Kundennummer darf nur Ziffern enthalten", nil
		}
	}

	// Check width (if defined)
	if width > 0 && len(tail) != width {
		return false, fmt.Sprintf("Kundennummer muss genau %d-stellig sein", width), nil
	}

	// Uniqueness check
	var comp Company
	q := s.db.WithContext(ctx).Where("customer_number = ?", num)
	if err := q.First(&comp).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return true, "", nil
		}
		return false, "Datenbankfehler", err
	}

	// Allow if same company (excludeID)
	if excludeID != 0 && comp.ID == excludeID {
		return true, "", nil
	}

	// Taken by another record
	return false, "Kundennummer bereits vergeben", nil
}

// MaybeLiftCustomerCounterFor raises the settings counter if num's numeric part is ahead.
func (s *Store) MaybeLiftCustomerCounterFor(ctx context.Context, num string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var s Settings
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&s).Error; err != nil {
			return err
		}
		if n, ok := parseNumericPart(s.CustomerNumberPrefix, num); ok && n > s.CustomerNumberCounter {
			return tx.Model(&Settings{}).Where("id = ?", s.ID).
				Update("customer_number_counter", n).Error
		}
		return nil
	})
}

func (s *Store) LoadSettingsForExportCtx(
	ctx context.Context,
	ownerID uint,
) (*Settings, error) {
	var settings Settings
	if err := s.db.WithContext(ctx).
		Where("owner_id = ?", ownerID).
		First(&settings).Error; err != nil {
		return nil, fmt.Errorf("load settings for export (owner %d): %w", ownerID, err)
	}
	return &settings, nil
}
