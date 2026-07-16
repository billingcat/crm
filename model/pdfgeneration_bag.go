//go:build !speedata

package model

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/boxesandglue/bagme/document"
)

// CreateZUGFeRDPDF generates the invoice PDF locally and in-process using
// boxesandglue/bagme. This is the default engine; it is selected for any build
// without the "speedata" tag. The speedata variant (remote publishing server)
// lives in pdfgeneration_speedata.go and is enabled via "-tags speedata".
//
// The signature is identical to the speedata variant so the caller
// (controller/invoicecontroller.go) stays unchanged.
//
// Two layouts are supported, selected per invoice:
//   - mode 1 (generic, no letterhead): an HTML/CSS reproduction of
//     assets/generic/layout.xml, in invoice_layout_generic.go.
//   - mode 2 (user letterhead + regions): the invoice is drawn on top of a
//     letterhead PDF at the template's region coordinates, in
//     invoice_layout_letterhead.go.
//
// The generic layout additionally supports a user stylesheet (mode 3/B1): an
// "invoice.css" in the owner's asset directory is appended after the built-in
// CSS and can restyle the fixed, documented HTML scaffold (see
// docs/invoice-css.md).

// AutoLayoutNote describes, for the UI, what the "Automatisch" letterhead
// choice renders with this build's PDF engine (see the speedata variant in
// pdfgeneration_speedata.go).
const AutoLayoutNote = "Verwendet das eingebaute Standard-Layout (DIN 5008) mit den Firmendaten aus den Einstellungen."

func (s *Store) CreateZUGFeRDPDF(inv *Invoice, ownerID uint, xmlpath string, pdfpath string, logger *slog.Logger) error {
	// Reuse the exact same computation as the embedded XML so the printed
	// amounts (net, per-rate tax, grand total) match the ZUGFeRD data.
	settings, err := s.LoadSettings(ownerID)
	if err != nil {
		return fmt.Errorf("load settings: %w", err)
	}
	company, err := s.LoadCompany(inv.CompanyID, ownerID)
	if err != nil {
		return fmt.Errorf("load company %d: %w", inv.CompanyID, err)
	}
	zi := createZUGFerdXML(inv, settings, company)

	// The CII XML was already written to xmlpath by WriteZUGFeRDXML. Embedding
	// it via WithZUGFeRD also switches the output to PDF/A-3b and adds the
	// required XMP extension schema.
	xmlData, err := os.ReadFile(xmlpath)
	if err != nil {
		return fmt.Errorf("read ZUGFeRD xml %q: %w", xmlpath, err)
	}

	d, err := document.New(pdfpath, document.WithZUGFeRD(xmlData, "EN 16931"))
	if err != nil {
		return fmt.Errorf("create pdf document: %w", err)
	}
	d.Title = fmt.Sprintf("Rechnung %s", inv.Number)
	d.Author = settings.CompanyName
	d.Language = "de"

	// Mode 2 (letterhead + regions) vs. mode 1 (generic). inv is loaded via
	// LoadInvoiceWithTemplate, so Template and its Regions are preloaded when the
	// invoice references a template.
	if inv.TemplateID != nil && inv.Template != nil {
		err = s.layoutLetterheadInvoice(d, inv, company, &zi, ownerID)
	} else {
		err = s.layoutGenericInvoice(d, inv, settings, company, &zi, ownerID, logger)
	}
	if err != nil {
		return err
	}

	if err = d.Finish(); err != nil {
		return fmt.Errorf("finish pdf: %w", err)
	}

	logger.Debug("generated invoice PDF via boxesandglue",
		"invoice_id", inv.ID, "owner_id", ownerID, "pdfpath", pdfpath)
	return nil
}
