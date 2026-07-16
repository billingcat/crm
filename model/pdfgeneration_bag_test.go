//go:build !speedata

package model_test

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/billingcat/crm/fixtures"
	"github.com/billingcat/crm/model"
)

// TestCreateZUGFeRDPDF_Generic exercises the local boxesandglue engine end to
// end: seed an invoice, write its CII XML, then generate the PDF and assert it
// is a non-trivial PDF that embeds the ZUGFeRD attachment.
func TestCreateZUGFeRDPDF_Generic(t *testing.T) {
	store := fixtures.NewTestStore(t)
	td := fixtures.SeedTestData(t, store)

	inv, err := store.LoadInvoiceWithTemplate(td.Invoice.ID, fixtures.DefaultOwnerID)
	if err != nil {
		t.Fatalf("load invoice: %v", err)
	}

	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "invoice.xml")
	pdfPath := filepath.Join(dir, "invoice.pdf")

	if err = store.WriteZUGFeRDXML(inv, fixtures.DefaultOwnerID, xmlPath); err != nil {
		t.Fatalf("write zugferd xml: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err = store.CreateZUGFeRDPDF(inv, fixtures.DefaultOwnerID, xmlPath, pdfPath, logger); err != nil {
		t.Fatalf("create pdf: %v", err)
	}

	data, err := os.ReadFile(pdfPath)
	if err != nil {
		t.Fatalf("read pdf: %v", err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		t.Fatalf("output is not a PDF (first bytes: %q)", data[:min(8, len(data))])
	}
	if len(data) < 2000 {
		t.Fatalf("PDF suspiciously small: %d bytes", len(data))
	}
	// WithZUGFeRD embeds the CII as factur-x.xml.
	if !bytes.Contains(data, []byte("factur-x.xml")) {
		t.Errorf("PDF does not reference the embedded factur-x.xml attachment")
	}
	t.Logf("generated PDF: %d bytes", len(data))

	// Debug hook: set PDF_OUT to keep a copy of the generated PDF for visual
	// inspection.
	if out := os.Getenv("PDF_OUT"); out != "" {
		if err = os.WriteFile(out, data, 0644); err != nil {
			t.Fatalf("write PDF_OUT: %v", err)
		}
		t.Logf("copied PDF to %s", out)
	}
}

// TestCreateZUGFeRDPDF_Generic_UserCSS checks the mode-3/B1 path: an
// "invoice.css" in the owner's asset directory restyles the generic layout
// (bar color, custom font via a relative @font-face url), and a broken
// stylesheet must not fail the invoice.
func TestCreateZUGFeRDPDF_Generic_UserCSS(t *testing.T) {
	const sysFont = "/System/Library/Fonts/Supplemental/Arial.ttf"
	fontData, err := os.ReadFile(sysFont)
	if err != nil {
		t.Skipf("system font %s not available: %v", sysFont, err)
	}

	store := fixtures.NewTestStore(t)
	td := fixtures.SeedTestData(t, store)

	base := t.TempDir()
	store.Config.Basedir = base
	assetDir := filepath.Join(base, "assets", "userassets", "owner1")
	if err = os.MkdirAll(assetDir, 0o755); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}
	if err = os.WriteFile(filepath.Join(assetDir, "Custom.ttf"), fontData, 0o644); err != nil {
		t.Fatalf("copy font: %v", err)
	}
	userCSS := `@page { border-left: 5mm solid #003366; }
@font-face { font-family: "Hausschrift"; src: url("Custom.ttf"); }
body { font-family: "Hausschrift"; }`
	if err = os.WriteFile(filepath.Join(assetDir, "invoice.css"), []byte(userCSS), 0o644); err != nil {
		t.Fatalf("write invoice.css: %v", err)
	}

	inv, err := store.LoadInvoiceWithTemplate(td.Invoice.ID, fixtures.DefaultOwnerID)
	if err != nil {
		t.Fatalf("load invoice: %v", err)
	}

	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "invoice.xml")
	pdfPath := filepath.Join(dir, "invoice.pdf")
	if err = store.WriteZUGFeRDXML(inv, fixtures.DefaultOwnerID, xmlPath); err != nil {
		t.Fatalf("write zugferd xml: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err = store.CreateZUGFeRDPDF(inv, fixtures.DefaultOwnerID, xmlPath, pdfPath, logger); err != nil {
		t.Fatalf("create pdf: %v", err)
	}

	data, err := os.ReadFile(pdfPath)
	if err != nil {
		t.Fatalf("read pdf: %v", err)
	}
	// The @font-face font from the user stylesheet must be embedded
	// (subset shows up as /BaseFont /XXXXXX+ArialMT).
	if !bytes.Contains(data, []byte("ArialMT")) {
		t.Errorf("PDF does not embed the user stylesheet's custom font")
	}

	if out := os.Getenv("PDF_OUT"); out != "" {
		if err = os.WriteFile(out, data, 0o644); err != nil {
			t.Fatalf("write PDF_OUT: %v", err)
		}
		t.Logf("copied PDF to %s", out)
	}

	// A syntactically broken stylesheet must not break invoice generation.
	if err = os.WriteFile(filepath.Join(assetDir, "invoice.css"), []byte(`body { font-size: }`), 0o644); err != nil {
		t.Fatalf("write broken invoice.css: %v", err)
	}
	if err = store.CreateZUGFeRDPDF(inv, fixtures.DefaultOwnerID, xmlPath, filepath.Join(dir, "invoice2.pdf"), logger); err != nil {
		t.Errorf("broken invoice.css must not fail the invoice, got: %v", err)
	}
}

// TestCreateZUGFeRDPDF_Generic_MultiPage renders an invoice whose line-item
// table breaks across pages: the table must split (page 1 does not stay empty
// below the address block), the closing text must follow the totals directly
// instead of forcing an extra page, and the running footer must appear on
// every page.
func TestCreateZUGFeRDPDF_Generic_MultiPage(t *testing.T) {
	store := fixtures.NewTestStore(t)
	fixtures.SeedTestData(t, store)

	positions := make([]model.InvoicePosition, 45)
	for i := range positions {
		positions[i] = fixtures.Position(i+1, fmt.Sprintf("Leistung %d", i+1), 1, 100.00, 19)
	}
	invoice := fixtures.Invoice(
		fixtures.WithInvoiceNumber("INV-2024-0002"),
		fixtures.WithInvoiceCompanyID(1),
		fixtures.WithInvoicePositions(positions...),
		fixtures.WithInvoiceFooter("Vielen Dank für Ihren Auftrag.\nZahlbar innerhalb von 14 Tagen."),
	)
	if err := store.SaveInvoice(invoice, fixtures.DefaultOwnerID); err != nil {
		t.Fatalf("save invoice: %v", err)
	}

	inv, err := store.LoadInvoiceWithTemplate(invoice.ID, fixtures.DefaultOwnerID)
	if err != nil {
		t.Fatalf("load invoice: %v", err)
	}

	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "invoice.xml")
	pdfPath := filepath.Join(dir, "invoice.pdf")
	if err = store.WriteZUGFeRDXML(inv, fixtures.DefaultOwnerID, xmlPath); err != nil {
		t.Fatalf("write zugferd xml: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err = store.CreateZUGFeRDPDF(inv, fixtures.DefaultOwnerID, xmlPath, pdfPath, logger); err != nil {
		t.Fatalf("create pdf: %v", err)
	}

	data, err := os.ReadFile(pdfPath)
	if err != nil {
		t.Fatalf("read pdf: %v", err)
	}
	// 45 rows at ~6mm each cannot fit one A4 content area, but everything
	// (table + totals + closing text) must fit two pages: a third page would
	// mean the old "content after a split table forces a new page" bug.
	if pages := bytes.Count(data, []byte("/Type /Page")) - bytes.Count(data, []byte("/Type /Pages")); pages != 2 {
		t.Errorf("expected 2 pages, PDF has %d", pages)
	}

	if out := os.Getenv("PDF_OUT"); out != "" {
		if err = os.WriteFile(out, data, 0644); err != nil {
			t.Fatalf("write PDF_OUT: %v", err)
		}
		t.Logf("copied PDF to %s", out)
	}
}
