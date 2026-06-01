package model

import (
	"bytes"
	"text/template"
)

// DefaultInvoiceMailSubject is used when no template is configured for an owner/company.
const DefaultInvoiceMailSubject = "Rechnung {{.Number}}"

// DefaultInvoiceMailBody is used when no template is configured for an owner/company.
const DefaultInvoiceMailBody = `Sehr geehrte Damen und Herren,

anbei erhalten Sie unsere Rechnung Nr. {{.Number}} vom {{.Date}} über {{.Amount}} EUR.
Fällig zum {{.DueDate}}.

Mit freundlichen Grüßen`

// InvoiceMailData holds the values exposed to invoice mail templates.
//
// Fields use English identifiers so they map cleanly onto Go template
// expressions like {{.Number}}; the human-facing UI documents them
// in German.
type InvoiceMailData struct {
	Number   string
	Date     string
	DueDate  string
	Amount   string
	Contact  string
	Company  string
	TaxTotal string
	NetTotal string
}

// InvoiceMailPlaceholders lists the placeholders shown in the editor help text.
// Keep in sync with InvoiceMailData fields and BuildInvoiceMailData.
var InvoiceMailPlaceholders = []struct {
	Name string
	Desc string
}{
	{"Number", "Rechnungsnummer"},
	{"Date", "Rechnungsdatum (TT.MM.JJJJ)"},
	{"DueDate", "Fälligkeitsdatum (TT.MM.JJJJ)"},
	{"Amount", "Bruttobetrag"},
	{"NetTotal", "Nettobetrag"},
	{"TaxTotal", "Summe Umsatzsteuer"},
	{"Contact", "Ansprechpartner aus der Rechnung"},
	{"Company", "Firmenname des Empfängers"},
}

// BuildInvoiceMailData maps an invoice + company to the template data.
func BuildInvoiceMailData(inv *Invoice, cpy *Company) InvoiceMailData {
	tax := inv.GrossTotal.Sub(inv.NetTotal)
	return InvoiceMailData{
		Number:   inv.Number,
		Date:     inv.Date.Format("02.01.2006"),
		DueDate:  inv.DueDate.Format("02.01.2006"),
		Amount:   inv.GrossTotal.Round(2).StringFixed(2),
		NetTotal: inv.NetTotal.Round(2).StringFixed(2),
		TaxTotal: tax.Round(2).StringFixed(2),
		Contact:  inv.ContactInvoice,
		Company:  cpy.Name,
	}
}

// RenderInvoiceMail returns the rendered subject + body for an invoice mail.
//
// Resolution per field (subject and body independently):
//   company override → owner default → hard-coded default.
//
// Empty fields fall through to the next layer, so a company can override only the
// subject while keeping the owner-wide body. Parse/exec errors on a template also
// fall through to the hard-coded default, so callers always get usable strings.
func (s *Store) RenderInvoiceMail(ownerID uint, inv *Invoice, cpy *Company) (subject, body string, err error) {
	data := BuildInvoiceMailData(inv, cpy)

	subjectTpl := DefaultInvoiceMailSubject
	bodyTpl := DefaultInvoiceMailBody

	if owner, oerr := s.LoadOwnerEmailTemplate(ownerID, EmailTemplateKindInvoice); oerr != nil {
		err = oerr
	} else if owner != nil {
		if owner.Subject != "" {
			subjectTpl = owner.Subject
		}
		if owner.Body != "" {
			bodyTpl = owner.Body
		}
	}

	if cpy.ID != 0 {
		if co, cerr := s.LoadCompanyEmailTemplate(ownerID, cpy.ID, EmailTemplateKindInvoice); cerr != nil {
			err = cerr
		} else if co != nil {
			if co.Subject != "" {
				subjectTpl = co.Subject
			}
			if co.Body != "" {
				bodyTpl = co.Body
			}
		}
	}

	subject = renderOrDefault(subjectTpl, DefaultInvoiceMailSubject, data)
	body = renderOrDefault(bodyTpl, DefaultInvoiceMailBody, data)
	return subject, body, err
}

func renderOrDefault(tpl, fallback string, data any) string {
	if out, ok := tryRender(tpl, data); ok {
		return out
	}
	if out, ok := tryRender(fallback, data); ok {
		return out
	}
	return fallback
}

func tryRender(tpl string, data any) (string, bool) {
	t, err := template.New("mail").Option("missingkey=zero").Parse(tpl)
	if err != nil {
		return "", false
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", false
	}
	return buf.String(), true
}
