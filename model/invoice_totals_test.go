package model_test

import (
	"testing"

	"github.com/billingcat/crm/fixtures"
	"github.com/billingcat/crm/model"
	"github.com/shopspring/decimal"
)

func TestInvoice_RecomputeTotals(t *testing.T) {
	tests := []struct {
		name          string
		positions     []model.InvoicePosition
		wantNet       string
		wantGross     string
		wantTaxCount  int
	}{
		{
			name:          "empty invoice",
			positions:     nil,
			wantNet:       "0",
			wantGross:     "0",
			wantTaxCount:  0,
		},
		{
			name:          "single position 19% tax",
			positions:     []model.InvoicePosition{fixtures.Position(1, "Service", 1, 100.00, 19)},
			wantNet:       "100",
			wantGross:     "119",
			wantTaxCount:  1,
		},
		{
			name:          "multiple positions same tax rate",
			positions:     fixtures.SamplePositions(),
			wantNet:       "1660",    // 8*120 + 2*100 + 1*500
			wantGross:     "1975.4",  // 1660 * 1.19
			wantTaxCount:  1,
		},
		{
			name: "mixed tax rates",
			positions: []model.InvoicePosition{
				fixtures.Position(1, "Standard", 1, 100.00, 19),
				fixtures.Position(2, "Reduced", 1, 100.00, 7),
			},
			wantNet:       "200",
			wantGross:     "226", // 119 + 107
			wantTaxCount:  2,
		},
		{
			name:          "zero tax (reverse charge)",
			positions:     fixtures.ZeroTaxPositions(),
			wantNet:       "2500", // 10*150 + 5*200
			wantGross:     "2500", // no tax
			wantTaxCount:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inv := fixtures.Invoice(fixtures.WithInvoicePositions(tt.positions...))

			wantNet := decimal.RequireFromString(tt.wantNet)
			wantGross := decimal.RequireFromString(tt.wantGross)

			if !inv.NetTotal.Equal(wantNet) {
				t.Errorf("NetTotal = %s, want %s", inv.NetTotal, wantNet)
			}
			if !inv.GrossTotal.Equal(wantGross) {
				t.Errorf("GrossTotal = %s, want %s", inv.GrossTotal, wantGross)
			}
			if len(inv.TaxAmounts) != tt.wantTaxCount {
				t.Errorf("TaxAmounts count = %d, want %d", len(inv.TaxAmounts), tt.wantTaxCount)
			}
		})
	}
}

func TestInvoice_SaveAndLoad(t *testing.T) {
	store := fixtures.NewTestStore(t)
	data := fixtures.SeedTestData(t, store)

	// Create a new invoice
	inv := fixtures.Invoice(
		fixtures.WithInvoiceCompanyID(data.Company.ID),
		fixtures.WithInvoiceNumber("TEST-001"),
		fixtures.WithInvoicePositions(fixtures.SamplePositions()...),
	)

	if err := store.SaveInvoice(inv, fixtures.DefaultOwnerID); err != nil {
		t.Fatalf("SaveInvoice failed: %v", err)
	}

	// Verify invoice was saved with ID
	if inv.ID == 0 {
		t.Fatal("Invoice ID should be non-zero after save")
	}

	// Load it back
	loaded, err := store.LoadInvoice(inv.ID, fixtures.DefaultOwnerID)
	if err != nil {
		t.Fatalf("LoadInvoice failed: %v", err)
	}

	// Verify data
	if loaded.Number != "TEST-001" {
		t.Errorf("Number = %q, want %q", loaded.Number, "TEST-001")
	}
	if len(loaded.InvoicePositions) != 3 {
		t.Errorf("InvoicePositions count = %d, want 3", len(loaded.InvoicePositions))
	}
	if loaded.Status != model.InvoiceStatusDraft {
		t.Errorf("Status = %q, want %q", loaded.Status, model.InvoiceStatusDraft)
	}
}

func TestInvoice_StatusTransitions(t *testing.T) {
	store := fixtures.NewTestStore(t)
	data := fixtures.SeedTestData(t, store)

	inv := fixtures.Invoice(
		fixtures.WithInvoiceCompanyID(data.Company.ID),
		fixtures.WithInvoicePositions(fixtures.SamplePositions()...),
	)
	if err := store.SaveInvoice(inv, fixtures.DefaultOwnerID); err != nil {
		t.Fatalf("SaveInvoice failed: %v", err)
	}

	// Draft -> Issued
	if err := store.MarkInvoiceIssued(inv.ID, fixtures.DefaultOwnerID, inv.Date); err != nil {
		t.Fatalf("MarkInvoiceIssued failed: %v", err)
	}

	loaded, _ := store.LoadInvoice(inv.ID, fixtures.DefaultOwnerID)
	if loaded.Status != model.InvoiceStatusIssued {
		t.Errorf("Status after issue = %q, want %q", loaded.Status, model.InvoiceStatusIssued)
	}

	// Verify totals were persisted on issue
	if loaded.NetTotal.IsZero() {
		t.Error("NetTotal should be non-zero after issuing")
	}

	// Issued -> Paid
	if err := store.MarkInvoicePaid(inv.ID, fixtures.DefaultOwnerID, inv.Date); err != nil {
		t.Fatalf("MarkInvoicePaid failed: %v", err)
	}

	loaded, _ = store.LoadInvoice(inv.ID, fixtures.DefaultOwnerID)
	if loaded.Status != model.InvoiceStatusPaid {
		t.Errorf("Status after paid = %q, want %q", loaded.Status, model.InvoiceStatusPaid)
	}
}
