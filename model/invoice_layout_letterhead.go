package model

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/boxesandglue/bagme/document"
	"github.com/speedata/einvoice"
)

// layoutLetterheadInvoice renders the invoice on top of a user-defined
// letterhead (mode 2). The layout is driven by the template's three regions
// (LetterheadTemplate.Regions, measured in cm from the top-left paper edge):
//
//   - main_area:     defines the @page margins; the line-item table flows here
//     and breaks across pages.
//   - addressee:     recipient address block, placed on page 1 at its region.
//   - invoice_info:  date / number / due date, placed on page 1 at its region.
//
// The letterhead PDF is painted as a full-page background on every page via a
// CSS `@page { background-image: url(...) }` rule; htmlbag loads the PDF,
// scales it to the sheet and draws it behind the content. When main_area has a
// distinct page-2 rectangle (HasPage2), later pages use that rectangle and PDF
// page 2 via `@page :first` vs. `@page` (see letterheadInvoiceCSS). The caller
// (CreateZUGFeRDPDF) owns document creation and calls Finish afterwards.
func (s *Store) layoutLetterheadInvoice(d *document.Document, inv *Invoice, company *Company, zi *einvoice.Invoice, ownerID uint) error {
	tpl := inv.Template

	pageW, pageH := tpl.PageWidthCm, tpl.PageHeightCm
	if pageW <= 0 || pageH <= 0 {
		pageW, pageH = 21.0, 29.7 // A4 fallback
	}

	main := findRegion(tpl.Regions, FieldPositions)
	addressee := findRegion(tpl.Regions, FieldSender)
	info := findRegion(tpl.Regions, FieldInvoiceInfo)

	assetDir := filepath.Join(s.Config.Basedir, "assets", "userassets", fmt.Sprintf("owner%d", ownerID))

	// Absolute path of the letterhead PDF (empty when the template has no PDF).
	// It becomes the CSS @page background; htmlbag resolves an absolute url()
	// path directly and skips (with a warning) when the file cannot be loaded,
	// so a missing PDF just yields a blank background — same as before.
	bgPath := ""
	if p := strings.TrimSpace(tpl.PDFPath); p != "" {
		bgPath = filepath.Join(assetDir, p)
	}

	if err := d.AddCSS(letterheadInvoiceCSS(pageW, pageH, main, addressee, info, bgPath)); err != nil {
		return fmt.Errorf("add css: %w", err)
	}
	// Custom template fonts, appended after the base CSS so the body
	// font-family override wins the cascade.
	if fontCSS := letterheadFontCSS(assetDir, tpl); fontCSS != "" {
		if err := d.AddCSS(fontCSS); err != nil {
			return fmt.Errorf("add font css: %w", err)
		}
	}

	// The addressee and invoice-info blocks are absolutely positioned against
	// the paper edge (position:absolute anchors to the page box); their region
	// coordinates apply verbatim in the CSS. They flow at the start of the
	// document, so they render on page 1 only. No sender/bank/footer blocks
	// here — the letterhead itself carries that branding.
	var b strings.Builder
	if addressee != nil {
		b.WriteString(`<div class="lh-addressee">` + buildAddresseeInnerHTML(inv, company) + `</div>`)
	}
	if info != nil {
		b.WriteString(`<div class="lh-info">` + buildInvoiceInfoInnerHTML(inv) + `</div>`)
	}
	b.WriteString(buildInvoiceBodyHTML(zi, inv))

	if err := d.RenderPages(b.String()); err != nil {
		return fmt.Errorf("render pages: %w", err)
	}
	return nil
}

// letterheadInvoiceCSS builds the stylesheet for the letterhead layout: the
// @page size/margins from the main_area region, the letterhead PDF as the @page
// background image, the body font from main_area, and the per-region
// font/alignment for the two positioned blocks. The shared invoiceItemsCSS
// styles the line-item table. bgPath is the absolute path of the letterhead PDF
// (empty for no background).
//
// When main_area carries a distinct page-2 rectangle (HasPage2, set in the
// editor only for two-page letterhead PDFs), page 1 uses rectangle 1 and PDF
// page 1 via `@page :first`, and all later pages use rectangle 2 and PDF page 2
// via the base `@page`. htmlbag applies the margins per page and reflows the
// running text (including split tables) at the page-2 content width.
func letterheadInvoiceCSS(pageW, pageH float64, main, addressee, info *PlacedRegion, bgPath string) string {
	// @page margins from the main_area region (cm from the paper edges). Fall
	// back to a 2cm frame when the region is missing.
	mTop, mRight, mBottom, mLeft := 2.0, 2.0, 2.0, 2.0
	mainFont, mainLine := 10.0, 1.2
	if main != nil {
		mTop = main.YCm
		mLeft = main.XCm
		mRight = clampNonNeg(pageW - main.XCm - main.WidthCm)
		mBottom = clampNonNeg(pageH - main.YCm - main.HeightCm)
		if main.FontSizePt > 0 {
			mainFont = main.FontSizePt
		}
		if main.LineSpacing > 0 {
			mainLine = main.LineSpacing
		}
	}

	// The letterhead PDF is painted as the full-page @page background. The
	// path is quoted so spaces survive the url() token; -bag-background-page
	// selects the source page of the PDF.
	bgFor := func(pdfPage int) string {
		if bgPath == "" {
			return ""
		}
		return fmt.Sprintf(" background-image: url(%q); -bag-background-page: %d;", bgPath, pdfPage)
	}

	var b strings.Builder
	if main != nil && main.HasPage2 {
		m2Top := main.Y2Cm
		m2Left := main.X2Cm
		m2Right := clampNonNeg(pageW - main.X2Cm - main.Width2Cm)
		m2Bottom := clampNonNeg(pageH - main.Y2Cm - main.Height2Cm)
		fmt.Fprintf(&b, "@page { size: %gcm %gcm; margin: %gcm %gcm %gcm %gcm;%s }\n",
			pageW, pageH, m2Top, m2Right, m2Bottom, m2Left, bgFor(2))
		fmt.Fprintf(&b, "@page :first { margin: %gcm %gcm %gcm %gcm;%s }\n",
			mTop, mRight, mBottom, mLeft, bgFor(1))
	} else {
		fmt.Fprintf(&b, "@page { size: %gcm %gcm; margin: %gcm %gcm %gcm %gcm;%s }\n",
			pageW, pageH, mTop, mRight, mBottom, mLeft, bgFor(1))
	}
	fmt.Fprintf(&b, "body { font-family: sans-serif; font-size: %gpt; line-height: %g; }\n",
		mainFont, mainLine)
	b.WriteString(regionBlockCSS("lh-addressee", addressee, "left"))
	b.WriteString(regionBlockCSS("lh-info", info, "right"))
	b.WriteString(invoiceItemsCSS)
	return b.String()
}

// letterheadFontFamily is the CSS family name under which the template's
// custom fonts are registered.
const letterheadFontFamily = "LetterheadFont"

// letterheadFontCSS builds @font-face rules for the template's custom fonts
// (FontNormal/FontBold/FontItalic, file names inside the owner's asset
// directory). Faces whose file is missing are skipped silently, and the body
// is switched to the custom family only when the normal face exists — so a
// template without (or with broken) fonts keeps the bagme default fonts.
// Returns "" when nothing usable is configured.
func letterheadFontCSS(assetDir string, tpl *LetterheadTemplate) string {
	faces := []struct {
		file, weight, style string
	}{
		{tpl.FontNormal, "normal", "normal"},
		{tpl.FontBold, "bold", "normal"},
		{tpl.FontItalic, "normal", "italic"},
	}
	var b strings.Builder
	for _, f := range faces {
		name := strings.TrimSpace(f.file)
		if name == "" {
			continue
		}
		p := filepath.Join(assetDir, filepath.Base(name))
		if _, err := os.Stat(p); err != nil {
			continue
		}
		fmt.Fprintf(&b, "@font-face { font-family: %q; src: url(%q); font-weight: %s; font-style: %s; }\n",
			letterheadFontFamily, p, f.weight, f.style)
	}
	if b.Len() == 0 {
		return ""
	}
	if n := strings.TrimSpace(tpl.FontNormal); n != "" {
		if _, err := os.Stat(filepath.Join(assetDir, filepath.Base(n))); err == nil {
			fmt.Fprintf(&b, "body { font-family: %q; }\n", letterheadFontFamily)
		}
	}
	return b.String()
}

// regionBlockCSS emits the CSS rule for one positioned region block: absolute
// position and width from the region's paper-edge cm coordinates
// (position:absolute anchors to the page box), plus font size, line spacing
// and horizontal alignment (with fallbacks). Returns an empty string when the
// template has no such region.
func regionBlockCSS(class string, r *PlacedRegion, defaultAlign string) string {
	if r == nil {
		return ""
	}
	font, line, align := 10.0, 1.2, defaultAlign
	if r.FontSizePt > 0 {
		font = r.FontSizePt
	}
	if r.LineSpacing > 0 {
		line = r.LineSpacing
	}
	if r.HAlign != "" {
		align = r.HAlign
	}
	return fmt.Sprintf(".%s { position: absolute; top: %gcm; left: %gcm; width: %gcm; font-size: %gpt; line-height: %g; text-align: %s; }\n",
		class, r.YCm, r.XCm, r.WidthCm, font, line, align)
}

// findRegion returns the region of the given kind, or nil if the template has none.
func findRegion(regs []PlacedRegion, kind FieldKind) *PlacedRegion {
	for i := range regs {
		if regs[i].Kind == kind {
			return &regs[i]
		}
	}
	return nil
}

func clampNonNeg(v float64) float64 {
	if v < 0 {
		return 0
	}
	return v
}
