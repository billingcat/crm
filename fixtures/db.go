package fixtures

import (
	"testing"

	"github.com/billingcat/crm/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NewTestStore creates an in-memory SQLite database with auto-migrated schema.
// Use this in tests to get a clean database for each test.
func NewTestStore(t *testing.T) *model.Store {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	// Auto-migrate all models
	err = db.AutoMigrate(
		&model.User{},
		&model.Company{},
		&model.Person{},
		&model.Invoice{},
		&model.InvoicePosition{},
		&model.Settings{},
		&model.ContactInfo{},
		&model.Note{},
		&model.Tag{},
		&model.TagLink{},
		&model.APIToken{},
		&model.SignupToken{},
		&model.RecentView{},
		&model.LetterheadTemplate{},
		&model.PlacedRegion{},
		&model.Invitation{},
		&model.AuditLog{},
		&model.EmailTemplate{},
	)
	if err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}

	cfg := &model.Config{
		Mode: "test",
	}

	return model.NewStoreFromDB(db, cfg)
}

// SeedTestData populates the store with a standard set of test data.
// Returns the created entities for use in tests.
func SeedTestData(t *testing.T, store *model.Store) *TestData {
	t.Helper()

	// Create settings first (required for invoice generation)
	settings := Settings()
	if err := store.SaveSettings(settings); err != nil {
		t.Fatalf("failed to seed settings: %v", err)
	}

	// Create user
	user := User()
	if err := store.CreateUser(user); err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	// Create company
	company := Company()
	if err := store.SaveCompany(company, DefaultOwnerID, nil); err != nil {
		t.Fatalf("failed to seed company: %v", err)
	}

	// Create person linked to company
	person := Person(WithPersonCompanyID(int(company.ID)))
	if err := store.SavePerson(person, DefaultOwnerID, nil); err != nil {
		t.Fatalf("failed to seed person: %v", err)
	}

	// Create invoice with positions
	invoice := Invoice(
		WithInvoiceCompanyID(company.ID),
		WithInvoicePositions(SamplePositions()...),
	)
	if err := store.SaveInvoice(invoice, DefaultOwnerID); err != nil {
		t.Fatalf("failed to seed invoice: %v", err)
	}

	return &TestData{
		User:     user,
		Company:  company,
		Person:   person,
		Invoice:  invoice,
		Settings: settings,
	}
}

// TestData holds all seeded entities for easy access in tests
type TestData struct {
	User     *model.User
	Company  *model.Company
	Person   *model.Person
	Invoice  *model.Invoice
	Settings *model.Settings
}
