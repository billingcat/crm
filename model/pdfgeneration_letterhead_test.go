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
	"github.com/boxesandglue/bagme/document"
)

// TestCreateZUGFeRDPDF_Letterhead exercises the mode-2 (letterhead) path: seed an
// invoice, attach a letterhead template with the three standard regions, drop a
// dummy letterhead PDF into the owner's asset dir, then generate the invoice PDF
// and assert it is a non-trivial PDF embedding the ZUGFeRD attachment.
func TestCreateZUGFeRDPDF_Letterhead(t *testing.T) {
	store := fixtures.NewTestStore(t)
	td := fixtures.SeedTestData(t, store)

	// Point the store at a temp basedir and drop a dummy letterhead PDF into the
	// owner's asset directory, where layoutLetterheadInvoice resolves PDFPath.
	base := t.TempDir()
	store.Config.Basedir = base
	assetDir := filepath.Join(base, "assets", "userassets", "owner1")
	if err := os.MkdirAll(assetDir, 0o755); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}
	writeDummyLetterhead(t, filepath.Join(assetDir, "letterhead.pdf"))

	tpl := fixtures.SeedLetterheadTemplate(t, store, "letterhead.pdf")
	fixtures.AttachTemplateToInvoice(t, store, td.Invoice, tpl.ID)

	inv, err := store.LoadInvoiceWithTemplate(td.Invoice.ID, fixtures.DefaultOwnerID)
	if err != nil {
		t.Fatalf("load invoice: %v", err)
	}
	if inv.Template == nil {
		t.Fatalf("template was not preloaded onto the invoice")
	}
	if len(inv.Template.Regions) != 3 {
		t.Fatalf("expected 3 regions, got %d", len(inv.Template.Regions))
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
	t.Logf("generated letterhead PDF: %d bytes", len(data))

	// Debug hook: set PDF_OUT to keep a copy for visual inspection.
	if out := os.Getenv("PDF_OUT"); out != "" {
		if err = os.WriteFile(out, data, 0o644); err != nil {
			t.Fatalf("write PDF_OUT: %v", err)
		}
		t.Logf("copied PDF to %s", out)
	}
}

// TestCreateZUGFeRDPDF_Letterhead_CustomFont checks that template fonts from
// the owner's asset directory are embedded via @font-face: the generated PDF
// must reference the custom font, and a template pointing at a missing font
// file must fall back to the default fonts without failing.
func TestCreateZUGFeRDPDF_Letterhead_CustomFont(t *testing.T) {
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

	tpl := fixtures.SeedLetterheadTemplate(t, store, "")
	tpl.FontNormal = "Custom.ttf"
	tpl.FontBold = "Missing-Bold.ttf" // must be skipped, not fail
	if err = store.SaveLetterheadTemplate(tpl, fixtures.DefaultOwnerID); err != nil {
		t.Fatalf("save template: %v", err)
	}
	fixtures.AttachTemplateToInvoice(t, store, td.Invoice, tpl.ID)

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
	// The embedded font subset shows up as /BaseFont /XXXXXX+ArialMT.
	if !bytes.Contains(data, []byte("ArialMT")) {
		t.Errorf("PDF does not embed the custom font (no ArialMT BaseFont found)")
	}

	if out := os.Getenv("PDF_OUT"); out != "" {
		if err = os.WriteFile(out, data, 0o644); err != nil {
			t.Fatalf("write PDF_OUT: %v", err)
		}
		t.Logf("copied PDF to %s", out)
	}
}

// TestCreateZUGFeRDPDF_Letterhead_Page2Geometry exercises the HasPage2 path:
// a two-page letterhead PDF and a main_area with a distinct (wider, taller)
// page-2 rectangle. The invoice must break across pages, with page 1 using
// rectangle 1 / PDF page 1 and page 2 using rectangle 2 / PDF page 2.
func TestCreateZUGFeRDPDF_Letterhead_Page2Geometry(t *testing.T) {
	store := fixtures.NewTestStore(t)
	td := fixtures.SeedTestData(t, store)

	base := t.TempDir()
	store.Config.Basedir = base
	assetDir := filepath.Join(base, "assets", "userassets", "owner1")
	if err := os.MkdirAll(assetDir, 0o755); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}
	writeDummyLetterhead2Pages(t, filepath.Join(assetDir, "letterhead.pdf"))

	tpl := fixtures.SeedLetterheadTemplate(t, store, "letterhead.pdf")
	// Give main_area a distinct page-2 rectangle: wider (1cm side margins
	// instead of 2cm) and taller (starts below a smaller page-2 header).
	mainUpdate := []model.PlacedRegion{{
		Kind: model.FieldPositions,
		XCm:  2.0, YCm: 10.0, WidthCm: 17.0, HeightCm: 15.0,
		HAlign: "left", FontSizePt: 10, LineSpacing: 1.2,
		HasPage2: true,
		X2Cm:     1.0, Y2Cm: 4.0, Width2Cm: 19.0, Height2Cm: 23.0,
	}}
	if err := store.UpdateLetterheadRegionsAndFonts(tpl.ID, fixtures.DefaultOwnerID, mainUpdate, nil, 0, 0); err != nil {
		t.Fatalf("update main_area region: %v", err)
	}
	fixtures.AttachTemplateToInvoice(t, store, td.Invoice, tpl.ID)

	// Enough positions to force a page break.
	positions := make([]model.InvoicePosition, 40)
	for i := range positions {
		positions[i] = fixtures.Position(i+1, fmt.Sprintf("Leistung %d", i+1), 1, 100.00, 19)
	}
	td.Invoice.InvoicePositions = positions
	td.Invoice.Footer = "Vielen Dank für Ihren Auftrag."
	if err := store.UpdateInvoice(td.Invoice, fixtures.DefaultOwnerID); err != nil {
		t.Fatalf("update invoice: %v", err)
	}

	inv, err := store.LoadInvoiceWithTemplate(td.Invoice.ID, fixtures.DefaultOwnerID)
	if err != nil {
		t.Fatalf("load invoice: %v", err)
	}
	main := findMainArea(t, inv)
	if !main.HasPage2 {
		t.Fatalf("main_area did not keep HasPage2")
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
	if pages := bytes.Count(data, []byte("/Type /Page")) - bytes.Count(data, []byte("/Type /Pages")); pages < 2 {
		t.Errorf("expected a multi-page invoice, PDF has %d page(s)", pages)
	}

	if out := os.Getenv("PDF_OUT"); out != "" {
		if err = os.WriteFile(out, data, 0o644); err != nil {
			t.Fatalf("write PDF_OUT: %v", err)
		}
		t.Logf("copied PDF to %s", out)
	}
}

// findMainArea returns the main_area region of the invoice's template.
func findMainArea(t *testing.T, inv *model.Invoice) *model.PlacedRegion {
	t.Helper()
	for i := range inv.Template.Regions {
		if inv.Template.Regions[i].Kind == model.FieldPositions {
			return &inv.Template.Regions[i]
		}
	}
	t.Fatalf("template has no main_area region")
	return nil
}

// writeDummyLetterhead2Pages renders a two-page letterhead PDF whose pages are
// visually distinct (page 1: full blue header band; page 2: slim gray band),
// so the per-page background selection is visible in PDF_OUT inspection.
func writeDummyLetterhead2Pages(t *testing.T, path string) {
	t.Helper()
	d, err := document.New(path)
	if err != nil {
		t.Fatalf("new letterhead doc: %v", err)
	}
	// The bands flow at the top of their page (page margin is 0), separated
	// by a forced page break. Absolutely positioned bands would all render on
	// page 1 here, so plain flow blocks carry the per-page decoration.
	if err = d.AddCSS(`@page{size:a4;margin:0} body{font-family:sans-serif}
		.band{background-color:#1d4ed8;color:white;font-size:20pt;padding:10mm 20mm}
		.band2{background-color:#6b7280;color:white;font-size:10pt;padding:3mm 20mm;page-break-before:always}`); err != nil {
		t.Fatalf("letterhead css: %v", err)
	}
	if err = d.RenderPages(`<div class="band">MUSTERKOPF GmbH</div>` +
		`<div class="band2">Musterkopf GmbH — Folgeseite</div>`); err != nil {
		t.Fatalf("letterhead render: %v", err)
	}
	if err = d.Finish(); err != nil {
		t.Fatalf("letterhead finish: %v", err)
	}
}

// writeDummyLetterhead renders a minimal letterhead PDF (a colored header band
// plus a footer line) so the mode-2 renderer has a background to place. The band
// must carry content and padding — an empty colored box would collapse (see the
// htmlbag facts in docs/pdf-boxesandglue-plan.md).
func writeDummyLetterhead(t *testing.T, path string) {
	t.Helper()
	d, err := document.New(path)
	if err != nil {
		t.Fatalf("new letterhead doc: %v", err)
	}
	if err = d.AddCSS(`@page{size:a4;margin:0} body{font-family:sans-serif}
		.band{position:absolute;top:0;left:0;width:210mm;background-color:#1d4ed8;color:white;font-size:20pt;padding:10mm 20mm}
		.foot{position:absolute;top:282mm;left:20mm;width:170mm;font-size:8pt;color:#333}`); err != nil {
		t.Fatalf("letterhead css: %v", err)
	}
	if err = d.RenderPages(`<div class="band">MUSTERKOPF GmbH</div>` +
		`<div class="foot">Musterkopf GmbH · Musterstr. 1 · 10115 Berlin</div><p> </p>`); err != nil {
		t.Fatalf("letterhead render: %v", err)
	}
	if err = d.Finish(); err != nil {
		t.Fatalf("letterhead finish: %v", err)
	}
}
