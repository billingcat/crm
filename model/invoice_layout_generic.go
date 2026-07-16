//go:build !speedata

package model

import (
	"fmt"
	"html"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/boxesandglue/bagme/document"
	"github.com/shopspring/decimal"
	"github.com/speedata/einvoice"
)

// genericInvoiceCSS reproduces the look of assets/generic/layout.xml (the
// speedata generic layout) closely, but not pixel-perfect: A4 with the same
// margins, a line-item table with rules above/below the header, right-aligned
// amounts, and a three-column footer (seller / bank / VAT id). The layout is
// pure HTML/CSS — no Go page decorations: the green left bar is the @page
// border-left, and the footer is a CSS running element repeated in the
// @bottom-center margin box on every page.
// genericBarColorRGB is the green left bar color, converted from the original
// CMYK 70/8/98/25 in layout.xml. RGB (not CMYK) because the PDF/A-3b output
// carries an sRGB output intent.
const genericBarColorRGB = "#39b004"

const genericInvoiceCSS = `
/* The 5mm green bar is the @page border-left, repeated on every page at the
   paper edge. It spans the full sheet height only with margin:0 (the border
   belongs to the page box, which margins would shrink), so the content is
   indented via padding instead: the content area starts 25mm left (5mm border
   + 20mm padding) and 10mm top from the paper edge; bottom 35mm keeps the
   flow clear of the footer. With margin:0 the @bottom-center margin box sits
   at the bottom paper edge with zero height, so the box's own margins place
   the footer: the negative margin-top raises its top edge 22mm above the
   paper edge (like the old Go-driven footer), left/right match the 20mm/10mm
   content indent. */
@page {
	size: a4;
	margin: 0;
	border-left: 5mm solid ` + genericBarColorRGB + `;
	padding: 10mm 10mm 35mm 20mm;
	@bottom-center { content: element(pagefooter); margin: -22mm 10mm 0 20mm; }
}
body { font-family: sans-serif; font-size: 10pt; }

/* DIN 5008 Form B header. position:absolute anchors to the page box (paper
   edge), so the DIN measures apply verbatim: Rücksendeangabe 45mm from top,
   Anschriftzone 63mm, both 25mm from the left edge; the invoice info block
   sits right-aligned against the 10mm right margin. A flow spacer reserves
   the vertical space so the line-item table starts below the address field
   on page 1. */
.sender-line { position: absolute; top: 45mm; left: 25mm; width: 85mm; font-size: 8pt; }
.addressee   { position: absolute; top: 63mm; left: 25mm; width: 80mm; }
.info        { position: absolute; top: 45mm; right: 10mm; width: 80mm; text-align: right; }
/* Reserve the address-field space on page 1. Only margin-top reliably creates
   vertical space here (empty height/padding collapse); it does not collapse at
   the page top, and later pages have no wrapper so the table flows to the top. */
.below-address { margin-top: 85mm; }

/* The footer is captured as a running element and re-emitted in the
   @bottom-center margin box on every page; the box's margins (see @page)
   position it, so the table just fills the box width. */
.pagefooter { position: running(pagefooter); }
table.foot { width: 100%; font-size: 8pt; }
table.foot td { vertical-align: top; }
` + invoiceItemsCSS

// invoiceItemsCSS styles the flowing invoice body: the opening/closing text and
// the line-item table with its totals rows. It is shared by the generic layout
// (mode 1, this file) and the letterhead layout (mode 2,
// invoice_layout_letterhead.go), which both render buildInvoiceBodyHTML.
const invoiceItemsCSS = `
p.opening { margin: 4mm 0; }

table.items { width: 100%; margin-top: 4mm; border-collapse: collapse; }
table.items th {
	border-top: 0.5pt solid black;
	border-bottom: 0.5pt solid black;
	padding: 2pt 4pt;
	font-style: italic;
	font-weight: normal;
}
table.items td { padding: 2pt 4pt; vertical-align: top; }
th.num, td.num { text-align: right; }
th.unit, td.unit { text-align: center; }
tr.sumfirst td { border-top: 1.5pt solid black; }
tr.total td { font-weight: bold; }
td.sumlabel { text-align: right; }
`

// buildGenericInvoiceHTML renders the invoice body as HTML for the generic
// (no-letterhead) layout. zi carries the computed totals and per-rate taxes so
// the printed amounts match the embedded ZUGFeRD XML exactly; inv/settings
// provide the remaining display data.
func buildGenericInvoiceHTML(zi *einvoice.Invoice, inv *Invoice, settings *Settings, company *Company) string {
	var b strings.Builder

	// --- page footer: captured as a CSS running element (no flow space) and
	// repeated in the @bottom-center margin box on every page. ---
	b.WriteString(`<footer class="pagefooter">`)
	b.WriteString(buildGenericFooterHTML(settings))
	b.WriteString(`</footer>`)

	// --- header (DIN 5008 Form B): sender line, addressee and invoice info as
	// absolutely positioned blocks; a flow spacer reserves the space so the
	// line-item table starts below the address field on page 1. ---
	b.WriteString(`<div class="sender-line">`)
	b.WriteString(esc(joinNonEmpty(" · ",
		settings.CompanyName, settings.Address1, settings.Address2,
		strings.TrimSpace(settings.ZIP+" "+settings.City))))
	b.WriteString(`</div>`)

	b.WriteString(`<div class="addressee">`)
	b.WriteString(buildAddresseeInnerHTML(inv, company))
	b.WriteString(`</div>`)

	b.WriteString(`<div class="info">`)
	b.WriteString(buildInvoiceInfoInnerHTML(inv))
	b.WriteString(`</div>`)

	// Everything below the address field flows in a wrapper whose margin-top
	// reserves the page-1 address space (see .below-address).
	b.WriteString(`<div class="below-address">`)
	b.WriteString(buildInvoiceBodyHTML(zi, inv))
	b.WriteString(`</div>`) // .below-address

	return b.String()
}

// buildAddresseeInnerHTML renders the recipient address block (company name,
// optional invoice contact, postal address) as inline HTML without a wrapping
// element. Shared by both layouts; the caller wraps and positions it.
func buildAddresseeInnerHTML(inv *Invoice, company *Company) string {
	var b strings.Builder
	b.WriteString(esc(company.Name))
	if inv.ContactInvoice != "" {
		b.WriteString("<br/>" + esc(inv.ContactInvoice))
	}
	if company.Address1 != "" {
		b.WriteString("<br/>" + esc(company.Address1))
	}
	if company.Address2 != "" {
		b.WriteString("<br/>" + esc(company.Address2))
	}
	b.WriteString("<br/>" + esc(strings.TrimSpace(company.Zip+" "+company.City)))
	return b.String()
}

// buildInvoiceInfoInnerHTML renders the invoice-info block (date, number, due
// date) as inline HTML without a wrapping element. Shared by both layouts.
func buildInvoiceInfoInnerHTML(inv *Invoice) string {
	var b strings.Builder
	b.WriteString("Datum: " + esc(formatDateDE(inv.Date)) + "<br/>")
	b.WriteString("Rechnung " + esc(inv.Number))
	if !inv.DueDate.IsZero() {
		b.WriteString("<br/>Zahlungsziel: " + esc(formatDateDE(inv.DueDate)))
	}
	return b.String()
}

// buildInvoiceBodyHTML renders the flowing part of the invoice: opening text,
// the line-item table with totals, and closing text. This is the content that
// breaks across pages and is shared by both layouts (styled via invoiceItemsCSS).
// zi carries the computed totals so the printed amounts match the embedded
// ZUGFeRD XML exactly.
func buildInvoiceBodyHTML(zi *einvoice.Invoice, inv *Invoice) string {
	currency := currencyCodeToText(inv.Currency)
	hasDifferentTax := len(zi.TradeTaxes) > 1
	// One extra "Steuer" column only when line items carry different rates.
	ncols := 5
	if hasDifferentTax {
		ncols = 6
	}

	var b strings.Builder

	// --- opening text ---
	if strings.TrimSpace(inv.Opening) != "" {
		b.WriteString(`<p class="opening">` + escMultiline(inv.Opening) + `</p>`)
	}

	// --- line-item table ---
	b.WriteString(`<table class="items"><thead><tr>`)
	b.WriteString(`<th class="num">Menge</th>`)
	b.WriteString(`<th class="unit">Einheit</th>`)
	b.WriteString(`<th>Leistung</th>`)
	if hasDifferentTax {
		b.WriteString(`<th class="num">Steuer</th>`)
	}
	b.WriteString(`<th class="num">Einzelpreis<br/>(` + esc(currency) + `)</th>`)
	b.WriteString(`<th class="num">Gesamtpreis<br/>(` + esc(currency) + `)</th>`)
	b.WriteString(`</tr></thead><tbody>`)

	for _, pos := range inv.InvoicePositions {
		b.WriteString(`<tr>`)
		b.WriteString(`<td class="num">` + esc(formatQuantityDE(pos.Quantity)) + `</td>`)
		b.WriteString(`<td class="unit">` + esc(unitCodeToText(pos.UnitCode)) + `</td>`)
		b.WriteString(`<td>` + esc(pos.Text) + `</td>`)
		if hasDifferentTax {
			b.WriteString(`<td class="num">` + esc(formatQuantityDE(pos.TaxRate)) + `%</td>`)
		}
		b.WriteString(`<td class="num">` + esc(formatAmountDE(pos.NetPrice)) + `</td>`)
		b.WriteString(`<td class="num">` + esc(formatAmountDE(pos.LineTotal)) + `</td>`)
		b.WriteString(`</tr>`)
	}

	// --- totals ---
	b.WriteString(sumRow("sumfirst", ncols, "Nettosumme", zi.LineTotal))
	for _, tt := range zi.TradeTaxes {
		label := taxCategoryText(tt.CategoryCode, formatQuantityDE(tt.Percent), tt.ExemptionReason)
		b.WriteString(sumRow("", ncols, label, tt.CalculatedAmount))
	}
	b.WriteString(sumRow("total", ncols, "Gesamtbetrag", zi.GrandTotal))
	b.WriteString(`</tbody></table>`)

	// --- closing text ---
	if strings.TrimSpace(inv.Footer) != "" {
		b.WriteString(`<p class="closing">` + escMultiline(inv.Footer) + `</p>`)
	}

	return b.String()
}

// userInvoiceCSSFile is the file name (inside the owner's asset directory,
// uploadable via the file manager) of the optional user stylesheet for the
// generic layout (mode 3/B1). The HTML scaffold and its class names are the
// documented, stable API for it; see docs/invoice-css.md.
const userInvoiceCSSFile = "invoice.css"

// layoutGenericInvoice renders the invoice with the generic, no-letterhead
// layout (mode 1): the DIN 5008 header, the flowing line-item table, the green
// left bar (@page border-left) and the footer (CSS running element) — pure
// HTML/CSS, no Go page decorations. An optional user stylesheet
// (userInvoiceCSSFile in the owner's asset directory) is appended after the
// built-in CSS, so its rules win the cascade; a broken user stylesheet is
// logged and skipped rather than failing the invoice. The caller
// (CreateZUGFeRDPDF) owns document creation and calls Finish afterwards.
func (s *Store) layoutGenericInvoice(d *document.Document, inv *Invoice, settings *Settings, company *Company, zi *einvoice.Invoice, ownerID uint, logger *slog.Logger) error {
	if err := d.AddCSS(genericInvoiceCSS); err != nil {
		return fmt.Errorf("add css: %w", err)
	}
	// ReadCSSFile (not AddCSS) so relative url() references — e.g. @font-face
	// sources — resolve against the asset directory the stylesheet lives in.
	cssPath := filepath.Join(s.Config.Basedir, "assets", "userassets", fmt.Sprintf("owner%d", ownerID), userInvoiceCSSFile)
	if _, err := os.Stat(cssPath); err == nil {
		if err = d.ReadCSSFile(cssPath); err != nil {
			logger.Warn("user invoice.css could not be applied, rendering with default styling",
				"err", err, "invoice_id", inv.ID, "owner_id", ownerID)
		}
	}
	if err := d.RenderPages(buildGenericInvoiceHTML(zi, inv, settings, company)); err != nil {
		return fmt.Errorf("render pages: %w", err)
	}
	return nil
}

// buildGenericFooterHTML renders the page footer (seller / bank / VAT id),
// mirroring the AtPageShipout footer in the speedata generic layout. It is
// wrapped as a running element and repeated in the @bottom-center margin box
// on every page (see genericInvoiceCSS).
func buildGenericFooterHTML(settings *Settings) string {
	var b strings.Builder
	b.WriteString(`<table class="foot"><tr><td>`)
	b.WriteString(esc(settings.CompanyName) + "<br/>")
	b.WriteString(esc(settings.Address1) + "<br/>")
	if settings.Address2 != "" {
		b.WriteString(esc(settings.Address2) + "<br/>")
	}
	b.WriteString(esc(strings.TrimSpace(settings.ZIP + " " + settings.City)))
	b.WriteString(`</td>`)
	if settings.BankIBAN != "" {
		b.WriteString(`<td>Bankverbindung<br/>`)
		if settings.BankName != "" {
			b.WriteString(esc(settings.BankName) + "<br/>")
		}
		b.WriteString(esc(formatIBAN(settings.BankIBAN)))
		b.WriteString(`</td>`)
	}
	b.WriteString(`<td>Umsatzsteuer-ID<br/>` + esc(settings.VATID) + `</td>`)
	b.WriteString(`</tr></table>`)
	return b.String()
}

// sumRow renders one totals row spanning all but the last column for the label.
func sumRow(class string, ncols int, label string, amount decimal.Decimal) string {
	attr := ""
	if class != "" {
		attr = ` class="` + class + `"`
	}
	return fmt.Sprintf(`<tr%s><td class="sumlabel" colspan="%d">%s</td><td class="num">%s</td></tr>`,
		attr, ncols-1, esc(label), esc(formatAmountDE(amount)))
}

// --- formatting helpers (mirror the sf:* functions in layout.xml) ---

func esc(s string) string { return html.EscapeString(s) }

// escMultiline escapes the text and turns newlines into <br/>.
func escMultiline(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = html.EscapeString(l)
	}
	return strings.Join(lines, "<br/>")
}

func joinNonEmpty(sep string, parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, sep)
}

// formatAmountDE formats a decimal as German currency: thousands separated by
// ".", two decimals after ",". Example: 1234.5 -> "1.234,50".
func formatAmountDE(d decimal.Decimal) string {
	s := d.StringFixed(2)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	intPart, frac, _ := strings.Cut(s, ".")

	var grouped strings.Builder
	n := len(intPart)
	for i := range n {
		if i > 0 && (n-i)%3 == 0 {
			grouped.WriteByte('.')
		}
		grouped.WriteByte(intPart[i])
	}
	res := grouped.String() + "," + frac
	if neg {
		res = "-" + res
	}
	return res
}

// formatQuantityDE prints a decimal without trailing zeros and with a comma
// as the decimal separator. Example: 8 -> "8", 2.50 -> "2,5".
func formatQuantityDE(d decimal.Decimal) string {
	return strings.Replace(d.String(), ".", ",", 1)
}

func formatDateDE(t interface{ Format(string) string }) string {
	return t.Format("02.01.2006")
}

// formatIBAN groups the IBAN into blocks of four characters.
func formatIBAN(iban string) string {
	iban = strings.ReplaceAll(iban, " ", "")
	var b strings.Builder
	for i, r := range iban {
		if i > 0 && i%4 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// currencyCodeToText mirrors sf:currency-code-to-text.
func currencyCodeToText(code string) string {
	switch code {
	case "EUR":
		return "Euro"
	case "USD":
		return "US-Dollar"
	default:
		return code
	}
}

// taxCategoryText mirrors sf:tax-category-code-to-text. rate is already
// formatted (no trailing zeros).
func taxCategoryText(code, rate, exemption string) string {
	switch code {
	case "S":
		return rate + "% Umsatzsteuer"
	case "AA":
		return rate + "% Ermäßigt A"
	case "B":
		return rate + "% Ermäßigt B"
	case "Z":
		return "Nullsatz"
	case "AE", "E":
		return exemption
	default:
		return code
	}
}

// unitCodeToText mirrors sf:unitcode-to-text (see erechnung.berlin/cii/unitcode).
func unitCodeToText(code string) string {
	switch code {
	case "C62", "H87":
		return "Stück"
	case "KGM":
		return "kg"
	case "LTR":
		return "Liter"
	case "MON":
		return "Monate"
	case "HUR":
		return "Stunden"
	case "MTK":
		return "m²"
	case "LS":
		return "pauschal"
	default:
		return code
	}
}
