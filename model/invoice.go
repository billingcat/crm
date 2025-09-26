package model

import (
	"database/sql"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/biter777/countries"
	"github.com/shopspring/decimal"
	"github.com/speedata/einvoice"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type InvoiceStatus string

const (
	InvoiceStatusDraft  InvoiceStatus = "draft"  // Entwurf
	InvoiceStatusIssued InvoiceStatus = "issued" // Gestellt/versendet
	InvoiceStatusPaid   InvoiceStatus = "paid"   // Bezahlt
	InvoiceStatusVoided InvoiceStatus = "voided" // Storniert
)

func (s InvoiceStatus) IsFinal() bool {
	return s == InvoiceStatusPaid || s == InvoiceStatusVoided
}

type Invoice struct {
	gorm.Model
	CompanyID        uint
	ContactInvoice   string
	Counter          uint
	Currency         string
	Date             time.Time
	DueDate          time.Time
	ExemptionReason  string
	Footer           string
	GrossTotal       decimal.Decimal
	InvoicePositions []InvoicePosition
	NetTotal         decimal.Decimal
	Number           string
	OccurrenceDate   time.Time
	Opening          string // Text before invoice
	OrderNumber      string
	OwnerID          uint
	SupplierNumber   string
	TaxAmounts       []TaxAmount `gorm:"-"`
	TaxNumber        string
	TaxType          string
	Status           InvoiceStatus `gorm:"type:text;not null;default:draft;check:status IN ('draft','issued','paid','voided');index;index:idx_owner_status"`
	IssuedAt         *time.Time    // gesetzt bei Status -> issued
	PaidAt           *time.Time    // gesetzt bei Status -> paid
	VoidedAt         *time.Time    // gesetzt bei Status -> voided
}

// TaxAmount collects the amount for each rate
type TaxAmount struct {
	Rate   decimal.Decimal
	Amount decimal.Decimal
}

// InvoicePosition contains one line in the invoice
type InvoicePosition struct {
	ID         uint `gorm:"primarykey"`
	CreatedAt  time.Time
	OwnerID    uint
	InvoiceID  uint
	Position   int
	UnitCode   string
	Text       string
	Quantity   decimal.Decimal `sql:"type:decimal(20,8);"`
	TaxRate    decimal.Decimal `sql:"type:decimal(20,8);"`
	NetPrice   decimal.Decimal `sql:"type:decimal(20,8);"`
	GrossPrice decimal.Decimal `sql:"type:decimal(20,8);"`
	LineTotal  decimal.Decimal `sql:"type:decimal(20,8);"`
}

func (InvoicePosition) TableName() string { return "invoicepositions" }

var hundred = decimal.NewFromInt(100)
var one = decimal.NewFromInt(1)

// SaveInvoice saves an invoice and all invoice positions
// SaveInvoice: robust gegen Duplikate
func (crmdb *CRMDatenbank) SaveInvoice(inv *Invoice, ownerid uint) error {
	return crmdb.db.Transaction(func(tx *gorm.DB) error {
		if inv.OwnerID != ownerid {
			return fmt.Errorf("save invoice: ownerid mismatch")
		}

		// 1) Rechnung speichern/erstellen (gehört immer zu ownerid)
		if err := tx.Save(inv).Error; err != nil {
			return err
		}

		// 2) Alte Positionen sicher entfernen (nur für diesen Owner)
		if err := tx.Where("invoice_id = ? AND owner_id = ?", inv.ID, ownerid).
			Delete(&InvoicePosition{}).Error; err != nil {
			return err
		}

		// 3) Neue Positionen sauber anlegen
		if len(inv.InvoicePositions) > 0 {
			for i := range inv.InvoicePositions {
				inv.InvoicePositions[i].ID = 0 // wichtig!
				inv.InvoicePositions[i].InvoiceID = inv.ID
				inv.InvoicePositions[i].OwnerID = ownerid // erzwingen
			}
			if err := tx.Omit("ID").Create(&inv.InvoicePositions).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// GetMaxCounter returns the maximum counter for the given company
func (crmdb *CRMDatenbank) GetMaxCounter(companyID uint, useLocalCounter bool, ownerID uint) (uint, error) {
	var max sql.NullInt64
	q := crmdb.db.Model(&Invoice{})
	if useLocalCounter {
		q = q.Where("company_id = ? AND owner_id = ?", companyID, ownerID)
	} else {
		q = q.Where("owner_id = ?", ownerID)
	}
	if err := q.Select("COALESCE(MAX(counter), 0)").Scan(&max).Error; err != nil {
		return 0, err
	}
	return uint(max.Int64), nil
}

// UpdateInvoice updates an invoice and fully replaces its positions (hard delete + recreate).
func (db *CRMDatenbank) UpdateInvoice(inv *Invoice, ownerid any) error {
	return db.db.Transaction(func(tx *gorm.DB) error {
		if inv.ID == 0 {
			return fmt.Errorf("update invoice: inv.ID is zero")
		}

		// 1) Invoice-Felder updaten (nur wenn OwnerID passt)
		if err := tx.Model(&Invoice{}).
			Where("id = ? AND owner_id = ?", inv.ID, ownerid).
			Updates(inv).Error; err != nil {
			return fmt.Errorf("update invoice: %w", err)
		}

		// 2) Alte Positionen löschen (nur wenn OwnerID passt)
		if err := tx.Where("invoice_id = ? AND owner_id = ?", inv.ID, ownerid).
			Delete(&InvoicePosition{}).Error; err != nil {
			return fmt.Errorf("delete positions: %w", err)
		}

		// 3) Neue Positionen anlegen
		if len(inv.InvoicePositions) > 0 {
			for i := range inv.InvoicePositions {
				inv.InvoicePositions[i].ID = 0
				inv.InvoicePositions[i].InvoiceID = inv.ID
				inv.InvoicePositions[i].OwnerID = ownerid.(uint) // falls dein Model das Feld hat
			}
			if err := tx.Omit("ID").Create(&inv.InvoicePositions).Error; err != nil {
				return fmt.Errorf("recreate positions: %w", err)
			}
		}

		return nil
	})
}

// DeleteInvoice removes an invoice and all referenced invoice positions from
// the database.
func (crmdb *CRMDatenbank) DeleteInvoice(inv *Invoice, ownerid any) error {
	// ensure we only delete invoices owned by the given owner
	result := crmdb.db.Where("owner_id = ?", ownerid).Select("InvoicePositions").Delete(inv)
	return result.Error
}

// LoadInvoice loads an invoice
func (crmdb *CRMDatenbank) LoadInvoice(id any, ownerid uint) (*Invoice, error) {
	var i Invoice
	// ensure invoice and its positions belong to the given owner
	result := crmdb.db.Where("owner_id = ?", ownerid).
		Preload("InvoicePositions", "owner_id = ?", ownerid).
		First(&i, id)
	if err := result.Error; err != nil {
		return nil, fmt.Errorf("load invoice %v: %w", id, result.Error)
	}
	calculateTaxAmounts(&i)

	return &i, result.Error
}

func calculateTaxAmounts(i *Invoice) {
	// reset previous amounts to avoid duplicates on repeated calls
	i.TaxAmounts = i.TaxAmounts[:0]
	totals := map[string]decimal.Decimal{}
	netTotal := decimal.Zero
	grossTotal := decimal.Zero

	for _, p := range i.InvoicePositions {
		if _, ok := totals[p.TaxRate.String()]; !ok {
			totals[p.TaxRate.String()] = decimal.Zero
		}
		taxrate := p.TaxRate.Div(hundred)
		netTotal = netTotal.Add(p.LineTotal)
		grossTotal = grossTotal.Add(p.LineTotal.Mul(taxrate.Add(one)))

		taxamount := p.LineTotal.Mul(taxrate)
		totals[p.TaxRate.String()] = totals[p.TaxRate.String()].Add(taxamount)
	}

	keys := make([]string, 0, len(totals))
	for k := range totals {
		keys = append(keys, k)
	}
	// sort strings as decimal
	sort.Slice(keys, func(i, j int) bool {
		di, _ := decimal.NewFromString(keys[i])
		dj, _ := decimal.NewFromString(keys[j])
		return di.LessThan(dj)
	})

	for _, key := range keys {
		ta := TaxAmount{
			Rate:   decimal.RequireFromString(key),
			Amount: totals[key],
		}
		i.TaxAmounts = append(i.TaxAmounts, ta)
	}
	i.GrossTotal = grossTotal
	i.NetTotal = netTotal
}

// countryID returns a to letter alpha code for the given country
func countryID(country string) string {
	c := countries.ByName(country)
	if c == countries.Unknown {
		return "DE" // default
	}
	return c.Alpha2()
}

func filterEmpty(ss ...string) []string {
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// CreateZUGFeRDXML writes the ZUGFeRD XML file to the hard drive. The file name
// is the invoice id plus the extension ".xml".
func (crmdb *CRMDatenbank) CreateZUGFeRDXML(inv *Invoice, ownerID any, path string) error {
	settings, err := crmdb.LoadSettings(ownerID)
	if err != nil {
		return err
	}
	company, err := crmdb.LoadCompany(inv.CompanyID, ownerID)
	if err != nil {
		return err
	}

	var sb strings.Builder

	// combine opening and footer, ignore empty lines
	// use a dot as separator, this is later replaced by a line break in the PDF
	// viewer
	text := strings.TrimSpace(strings.Join(
		filterEmpty(inv.Opening, inv.Footer), "·"))

	zi := einvoice.Invoice{
		InvoiceNumber:       inv.Number,
		InvoiceTypeCode:     380,
		Profile:             einvoice.CProfileEN16931,
		InvoiceDate:         inv.Date,
		OccurrenceDateTime:  inv.OccurrenceDate,
		InvoiceCurrencyCode: inv.Currency,
		TaxCurrencyCode:     inv.Currency,
		Notes: []einvoice.Note{{
			Text: text,
		}},
		Seller: einvoice.Party{
			Name:              settings.CompanyName,
			VATaxRegistration: settings.VATID,
			PostalAddress: &einvoice.PostalAddress{
				Line1:        settings.Address1,
				Line2:        settings.Address2,
				City:         settings.City,
				PostcodeCode: settings.ZIP,
				CountryID:    countryID(settings.CountryCode),
			},
			DefinedTradeContact: []einvoice.DefinedTradeContact{{
				PersonName: settings.InvoiceContact,
				EMail:      settings.InvoiceEMail,
			}},
		},
		Buyer: einvoice.Party{
			Name: company.Name,
			PostalAddress: &einvoice.PostalAddress{
				Line1:        company.Adresse1,
				Line2:        company.Adresse2,
				City:         company.Ort,
				PostcodeCode: company.PLZ,
				CountryID:    countryID(company.Land),
			},
			DefinedTradeContact: []einvoice.DefinedTradeContact{{
				PersonName: inv.ContactInvoice,
			}},
			VATaxRegistration: company.VATID,
		},
		PaymentMeans: []einvoice.PaymentMeans{
			{
				TypeCode:                                      30,
				PayeePartyCreditorFinancialAccountIBAN:        settings.BankIBAN,
				PayeePartyCreditorFinancialAccountName:        settings.BankName,
				PayeeSpecifiedCreditorFinancialInstitutionBIC: settings.BankBIC,
			},
		},
		SpecifiedTradePaymentTerms: []einvoice.SpecifiedTradePaymentTerms{{
			DueDate: inv.DueDate,
		}},
	}

	for _, pos := range inv.InvoicePositions {
		li := einvoice.InvoiceLine{
			LineID:                   fmt.Sprintf("%d", pos.Position),
			ItemName:                 pos.Text,
			BilledQuantity:           pos.Quantity,
			BilledQuantityUnit:       pos.UnitCode,
			NetPrice:                 pos.NetPrice,
			TaxRateApplicablePercent: pos.TaxRate,
			Total:                    pos.LineTotal,
			TaxTypeCode:              "VAT",
			TaxCategoryCode:          company.InvoiceTaxType,
		}
		zi.InvoiceLines = append(zi.InvoiceLines, li)
	}
	zi.UpdateApplicableTradeTax(map[string]string{"AE": inv.ExemptionReason})
	zi.UpdateTotals()

	err = zi.Write(&sb)
	if err != nil {
		return err
	}

	return os.WriteFile(path, []byte(sb.String()), 0644)

}

// --- Status Transitions ------------------------------------------------------
//
// Allowed transitions:
//   draft  -> issued | voided
//   issued -> paid   | voided
//   paid   -> (final, no further changes)
//   voided -> (final, no further changes)

func (crmdb *CRMDatenbank) changeInvoiceStatus(
	id uint, ownerID uint,
	to InvoiceStatus, t time.Time,
) error {
	return crmdb.db.Transaction(func(tx *gorm.DB) error {
		var inv Invoice

		// Lock the row (Postgres: FOR UPDATE; SQLite: no-op)
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND owner_id = ?", id, ownerID).
			First(&inv).Error; err != nil {
			return err
		}

		from := inv.Status

		// Guard: do not change final states
		if from.IsFinal() {
			return nil
		}

		// Allowed transitions map
		allowed := map[InvoiceStatus]map[InvoiceStatus]bool{
			InvoiceStatusDraft:  {InvoiceStatusIssued: true, InvoiceStatusVoided: true},
			InvoiceStatusIssued: {InvoiceStatusPaid: true, InvoiceStatusVoided: true},
		}
		if _, ok := allowed[from][to]; !ok {
			return fmt.Errorf("invalid status transition %q -> %q", from, to)
		}

		// Prepare fields to update
		updates := map[string]any{
			"status": to,
		}
		switch to {
		case InvoiceStatusIssued:
			updates["issued_at"] = t
			// Optional: assign invoice number here if needed
		case InvoiceStatusPaid:
			updates["paid_at"] = t
		case InvoiceStatusVoided:
			// Prevent voiding already paid invoices
			if from == InvoiceStatusPaid {
				return fmt.Errorf("paid invoices cannot be voided")
			}
			updates["voided_at"] = t
		}

		// Perform the update
		if err := tx.Model(&Invoice{}).
			Where("id = ? AND owner_id = ?", id, ownerID).
			Updates(updates).Error; err != nil {
			return err
		}

		return nil
	})
}

// In deinem model (z. B. in invoice.go):

// MarkInvoiceDraft rolls back an issued invoice to draft.
// Business rules: clears IssuedAt (and optionally Number/Counter).
func (crmdb *CRMDatenbank) MarkInvoiceDraft(id uint, ownerID uint, t time.Time) error {
	return crmdb.db.Transaction(func(tx *gorm.DB) error {
		var inv Invoice
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND owner_id = ?", id, ownerID).
			First(&inv).Error; err != nil {
			return err
		}

		// Only allow issued -> draft (paid/voided remain final)
		if inv.Status == InvoiceStatusPaid || inv.Status == InvoiceStatusVoided {
			return fmt.Errorf("cannot revert from %s to draft", inv.Status)
		}
		if inv.Status != InvoiceStatusIssued && inv.Status != InvoiceStatusDraft {
			// From draft to draft is a no-op
			if inv.Status == InvoiceStatusDraft {
				return nil
			}
			return fmt.Errorf("invalid transition %q -> draft", inv.Status)
		}

		updates := map[string]any{
			"status":    InvoiceStatusDraft,
			"issued_at": nil,
		}

		// Optional (falls du Nummern erst bei 'issued' vergibst und beim Rückgang löschen willst):
		// updates["number"]  = ""   // vorsichtig: nur wenn Nummer noch nicht an Kunden ging
		// updates["counter"] = 0    // dito

		return tx.Model(&Invoice{}).
			Where("id = ? AND owner_id = ?", id, ownerID).
			Updates(updates).Error
	})
}

// Convenience: draft -> issued
func (crmdb *CRMDatenbank) MarkInvoiceIssued(id uint, ownerID uint, t time.Time) error {
	return crmdb.changeInvoiceStatus(id, ownerID, InvoiceStatusIssued, t)
}

// Convenience: (draft|issued) -> paid
func (crmdb *CRMDatenbank) MarkInvoicePaid(id uint, ownerID uint, t time.Time) error {
	return crmdb.changeInvoiceStatus(id, ownerID, InvoiceStatusPaid, t)
}

// Convenience: (draft|issued) -> voided
func (crmdb *CRMDatenbank) VoidInvoice(id uint, ownerID uint, t time.Time) error {
	return crmdb.changeInvoiceStatus(id, ownerID, InvoiceStatusVoided, t)
}
