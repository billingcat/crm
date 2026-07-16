// Package fixtures provides factory functions for creating test fixtures.
// Use these in tests to construct valid domain objects without manual setup.
package fixtures

import (
	"time"

	"github.com/billingcat/crm/model"
	"github.com/shopspring/decimal"
)

// Option pattern for flexible fixture customization
type UserOption func(*model.User)
type CompanyOption func(*model.Company)
type PersonOption func(*model.Person)
type InvoiceOption func(*model.Invoice)
type SettingsOption func(*model.Settings)

// DefaultOwnerID is used for all fixtures unless overridden
const DefaultOwnerID uint = 1

// --- User ---

func User(opts ...UserOption) *model.User {
	u := &model.User{
		Email:    "test@example.com",
		FullName: "Test User",
		Password: "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ012", // bcrypt placeholder
		Verified: true,
		OwnerID:  DefaultOwnerID,
	}
	for _, opt := range opts {
		opt(u)
	}
	return u
}

func WithUserEmail(email string) UserOption {
	return func(u *model.User) { u.Email = email }
}

func WithUserName(name string) UserOption {
	return func(u *model.User) { u.FullName = name }
}

func WithUserOwnerID(id uint) UserOption {
	return func(u *model.User) { u.OwnerID = id }
}

// --- Company ---

func Company(opts ...CompanyOption) *model.Company {
	c := &model.Company{
		Name:           "Muster GmbH",
		Address1:       "Musterstraße 1",
		City:           "Berlin",
		Zip:            "10115",
		Country:        "Germany",
		VATID:          "DE123456789",
		OwnerID:        DefaultOwnerID,
		DefaultTaxRate: decimal.NewFromInt(19),
		InvoiceTaxType: "S", // Standard rate
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func WithCompanyName(name string) CompanyOption {
	return func(c *model.Company) { c.Name = name }
}

func WithCompanyAddress(addr1, zip, city, country string) CompanyOption {
	return func(c *model.Company) {
		c.Address1 = addr1
		c.Zip = zip
		c.City = city
		c.Country = country
	}
}

func WithCompanyVATID(vatid string) CompanyOption {
	return func(c *model.Company) { c.VATID = vatid }
}

func WithCompanyOwnerID(id uint) CompanyOption {
	return func(c *model.Company) { c.OwnerID = id }
}

func WithCompanyCustomerNumber(num string) CompanyOption {
	return func(c *model.Company) { c.CustomerNumber = num }
}

func WithCompanyTaxType(taxType string) CompanyOption {
	return func(c *model.Company) { c.InvoiceTaxType = taxType }
}

// IntraComCompany returns a company configured for intra-community supply (reverse charge)
func IntraComCompany(opts ...CompanyOption) *model.Company {
	c := Company(
		WithCompanyName("EU Partner B.V."),
		WithCompanyAddress("Keizersgracht 123", "1015 CJ", "Amsterdam", "Netherlands"),
		WithCompanyVATID("NL123456789B01"),
		WithCompanyTaxType("K"), // Intra-community
	)
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// --- Person ---

func Person(opts ...PersonOption) *model.Person {
	p := &model.Person{
		Name:     "Max Mustermann",
		EMail:    "max@example.com",
		Position: "Geschäftsführer",
		OwnerID:  DefaultOwnerID,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func WithPersonName(name string) PersonOption {
	return func(p *model.Person) { p.Name = name }
}

func WithPersonEmail(email string) PersonOption {
	return func(p *model.Person) { p.EMail = email }
}

func WithPersonCompanyID(id int) PersonOption {
	return func(p *model.Person) { p.CompanyID = id }
}

func WithPersonOwnerID(id uint) PersonOption {
	return func(p *model.Person) { p.OwnerID = id }
}

// --- Invoice ---

func Invoice(opts ...InvoiceOption) *model.Invoice {
	now := time.Now()
	inv := &model.Invoice{
		Number:         "INV-2024-0001",
		Date:           now,
		DueDate:        now.AddDate(0, 0, 14),
		OccurrenceDate: now,
		Currency:       "EUR",
		TaxType:        "S", // Standard rate
		Status:         model.InvoiceStatusDraft,
		OwnerID:        DefaultOwnerID,
		Counter:        1,
	}
	for _, opt := range opts {
		opt(inv)
	}
	return inv
}

func WithInvoiceNumber(num string) InvoiceOption {
	return func(i *model.Invoice) { i.Number = num }
}

func WithInvoiceDate(d time.Time) InvoiceOption {
	return func(i *model.Invoice) {
		i.Date = d
		i.OccurrenceDate = d
	}
}

func WithInvoiceDueDate(d time.Time) InvoiceOption {
	return func(i *model.Invoice) { i.DueDate = d }
}

func WithInvoiceCompanyID(id uint) InvoiceOption {
	return func(i *model.Invoice) { i.CompanyID = id }
}

func WithInvoiceOwnerID(id uint) InvoiceOption {
	return func(i *model.Invoice) { i.OwnerID = id }
}

func WithInvoiceStatus(s model.InvoiceStatus) InvoiceOption {
	return func(i *model.Invoice) { i.Status = s }
}

func WithInvoiceTaxType(t string) InvoiceOption {
	return func(i *model.Invoice) { i.TaxType = t }
}

func WithInvoicePositions(positions ...model.InvoicePosition) InvoiceOption {
	return func(i *model.Invoice) {
		i.InvoicePositions = positions
		i.RecomputeTotals()
	}
}

func WithInvoiceOpening(text string) InvoiceOption {
	return func(i *model.Invoice) { i.Opening = text }
}

func WithInvoiceFooter(text string) InvoiceOption {
	return func(i *model.Invoice) { i.Footer = text }
}

// --- Invoice Position ---

func Position(pos int, text string, qty, netPrice float64, taxRate int) model.InvoicePosition {
	q := decimal.NewFromFloat(qty)
	np := decimal.NewFromFloat(netPrice)
	tr := decimal.NewFromInt(int64(taxRate))
	lineTotal := q.Mul(np)

	return model.InvoicePosition{
		Position:   pos,
		Text:       text,
		Quantity:   q,
		NetPrice:   np,
		TaxRate:    tr,
		LineTotal:  lineTotal,
		GrossPrice: np.Mul(decimal.NewFromInt(1).Add(tr.Div(decimal.NewFromInt(100)))),
		UnitCode:   "HUR", // Hour
		OwnerID:    DefaultOwnerID,
	}
}

// PositionPiece creates a position with unit code "C62" (piece/unit)
func PositionPiece(pos int, text string, qty, netPrice float64, taxRate int) model.InvoicePosition {
	p := Position(pos, text, qty, netPrice, taxRate)
	p.UnitCode = "C62"
	return p
}

// SamplePositions returns a typical set of invoice positions for testing
func SamplePositions() []model.InvoicePosition {
	return []model.InvoicePosition{
		Position(1, "Software Development", 8, 120.00, 19),
		Position(2, "Project Management", 2, 100.00, 19),
		PositionPiece(3, "License Fee", 1, 500.00, 19),
	}
}

// ZeroTaxPositions returns positions with 0% tax (for reverse charge/intra-community)
func ZeroTaxPositions() []model.InvoicePosition {
	return []model.InvoicePosition{
		Position(1, "Consulting Services", 10, 150.00, 0),
		Position(2, "Implementation", 5, 200.00, 0),
	}
}

// --- Settings ---

func Settings(opts ...SettingsOption) *model.Settings {
	s := &model.Settings{
		OwnerID:               DefaultOwnerID,
		CompanyName:           "Testfirma GmbH",
		Address1:              "Teststraße 42",
		ZIP:                   "12345",
		City:                  "Teststadt",
		CountryCode:           "DE",
		VATID:                 "DE987654321",
		TAXNumber:             "123/456/78901",
		InvoiceNumberTemplate: "INV-{YYYY}-{NNNN}",
		BankIBAN:              "DE89370400440532013000",
		BankBIC:               "COBADEFFXXX",
		BankName:              "Test Bank",
		InvoiceContact:        "Buchhaltung",
		InvoiceEMail:          "invoice@testfirma.de",
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func WithSettingsOwnerID(id uint) SettingsOption {
	return func(s *model.Settings) { s.OwnerID = id }
}

func WithSettingsCompanyName(name string) SettingsOption {
	return func(s *model.Settings) { s.CompanyName = name }
}

func WithSettingsVATID(vatid string) SettingsOption {
	return func(s *model.Settings) { s.VATID = vatid }
}

func WithSettingsBank(iban, bic, name string) SettingsOption {
	return func(s *model.Settings) {
		s.BankIBAN = iban
		s.BankBIC = bic
		s.BankName = name
	}
}

func WithSettingsCustomerNumberFormat(prefix string, width int) SettingsOption {
	return func(s *model.Settings) {
		s.CustomerNumberPrefix = prefix
		s.CustomerNumberWidth = width
	}
}

// --- Note ---

type NoteOption func(*model.Note)

func Note(opts ...NoteOption) *model.Note {
	n := &model.Note{
		OwnerID:    DefaultOwnerID,
		AuthorID:   DefaultOwnerID,
		ParentType: model.ParentTypeCompany,
		ParentID:   1,
		Title:      "Test Notiz",
		Body:       "Dies ist eine Testnotiz.",
		Tags:       "test,wichtig",
	}
	for _, opt := range opts {
		opt(n)
	}
	return n
}

func WithNoteTitle(title string) NoteOption {
	return func(n *model.Note) { n.Title = title }
}

func WithNoteBody(body string) NoteOption {
	return func(n *model.Note) { n.Body = body }
}

func WithNoteTags(tags string) NoteOption {
	return func(n *model.Note) { n.Tags = tags }
}

func WithNoteAuthorID(id uint) NoteOption {
	return func(n *model.Note) { n.AuthorID = id }
}

func WithNoteOwnerID(id uint) NoteOption {
	return func(n *model.Note) { n.OwnerID = id }
}

func WithNoteParent(parentType model.ParentType, parentID uint) NoteOption {
	return func(n *model.Note) {
		n.ParentType = parentType
		n.ParentID = parentID
	}
}

// NoteForCompany creates a note attached to a company
func NoteForCompany(companyID uint, opts ...NoteOption) *model.Note {
	allOpts := append([]NoteOption{WithNoteParent(model.ParentTypeCompany, companyID)}, opts...)
	return Note(allOpts...)
}

// NoteForPerson creates a note attached to a person
func NoteForPerson(personID uint, opts ...NoteOption) *model.Note {
	allOpts := append([]NoteOption{WithNoteParent(model.ParentTypePerson, personID)}, opts...)
	return Note(allOpts...)
}
