package controller

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/billingcat/crm/model"
)

func (ctrl *controller) exportInvoicesXML(ctx context.Context, zw *zip.Writer, ownerID uint) error {
	invs, err := ctrl.model.ListInvoicesForExport(ownerID)
	if err != nil {
		return fmt.Errorf("cannot load invoices for export: %w", err)
	}

	// Create a new file in the ZIP archive
	f, err := zw.Create("invoices.xml")
	if err != nil {
		return fmt.Errorf("cannot create invoices.xml in ZIP: %w", err)
	}

	export := ExportInvoices{
		Version: "1",
	}

	export.Invoices = make([]APIInvoice, 0, len(invs))

	for i := range invs {
		inv := &invs[i]

		// Ensure totals and tax amounts are in sync with positions
		inv.RecomputeTotals()

		apiInv := ctrl.toAPIInvoice(inv)
		export.Invoices = append(export.Invoices, apiInv)
	}

	enc := xml.NewEncoder(f)
	enc.Indent("", "  ")
	if err := enc.Encode(export); err != nil {
		return fmt.Errorf("cannot encode invoices.xml: %w", err)
	}
	if err := enc.Flush(); err != nil {
		return fmt.Errorf("cannot flush invoices.xml: %w", err)
	}

	return nil
}

// toAPIInvoice maps a model.Invoice (with preloaded positions and tax amounts)
// to the APIInvoice struct used both by the JSON API and XML export.
func (ctrl *controller) toAPIInvoice(inv *model.Invoice) APIInvoice {
	positions := make([]APIInvoicePosition, len(inv.InvoicePositions))
	for i, p := range inv.InvoicePositions {
		positions[i] = APIInvoicePosition{
			ID:         p.ID,
			Position:   p.Position,
			UnitCode:   p.UnitCode,
			Text:       p.Text,
			Quantity:   p.Quantity.String(),
			TaxRate:    p.TaxRate.String(),
			NetPrice:   p.NetPrice.String(),
			GrossPrice: p.GrossPrice.String(),
			LineTotal:  p.LineTotal.String(),
		}
	}

	taxAmounts := make([]APITaxAmount, len(inv.TaxAmounts))
	for i, t := range inv.TaxAmounts {
		taxAmounts[i] = APITaxAmount{
			Rate:   t.Rate.String(),
			Amount: t.Amount.String(),
		}
	}

	return APIInvoice{
		ID:               inv.ID,
		Number:           inv.Number,
		Status:           string(inv.Status),
		Currency:         inv.Currency,
		NetTotal:         inv.NetTotal.String(),
		GrossTotal:       inv.GrossTotal.String(),
		Date:             inv.Date,
		DueDate:          inv.DueDate,
		CompanyID:        inv.CompanyID,
		ContactInvoice:   inv.ContactInvoice,
		Counter:          inv.Counter,
		ExemptionReason:  inv.ExemptionReason,
		Footer:           inv.Footer,
		Opening:          inv.Opening,
		OccurrenceDate:   inv.OccurrenceDate,
		OrderNumber:      inv.OrderNumber,
		BuyerReference:   inv.BuyerReference,
		SupplierNumber:   inv.SupplierNumber,
		TaxNumber:        inv.TaxNumber,
		TaxType:          inv.TaxType,
		TemplateID:       inv.TemplateID,
		IssuedAt:         inv.IssuedAt,
		PaidAt:           inv.PaidAt,
		VoidedAt:         inv.VoidedAt,
		CreatedAt:        inv.CreatedAt,
		UpdatedAt:        inv.UpdatedAt,
		InvoicePositions: positions,
		TaxAmounts:       taxAmounts,
	}
}

func (ctrl *controller) toAPICustomer(c *model.Company) APICustomer {
	contactInfos := make([]APIContactInfo, len(c.ContactInfos))
	for i := range c.ContactInfos {
		contactInfos[i] = ctrl.toAPIContactInfo(&c.ContactInfos[i])
	}

	notes := make([]APINote, len(c.Notes))
	for i := range c.Notes {
		notes[i] = ctrl.toAPINote(&c.Notes[i])
	}

	return APICustomer{
		ID:                     c.ID,
		Name:                   c.Name,
		CustomerNumber:         c.CustomerNumber,
		Address1:               c.Address1,
		Address2:               c.Address2,
		Zip:                    c.Zip,
		City:                   c.City,
		Country:                c.Country,
		InvoiceEmail:           c.InvoiceEmail,
		ContactInvoice:         c.ContactInvoice,
		SupplierNumber:         c.SupplierNumber,
		VATID:                  c.VATID,
		Background:             c.Background,
		DefaultTaxRate:         c.DefaultTaxRate.String(), // decimal.Decimal → String
		InvoiceCurrency:        c.InvoiceCurrency,
		InvoiceTaxType:         c.InvoiceTaxType,
		InvoiceOpening:         c.InvoiceOpening,
		InvoiceFooter:          c.InvoiceFooter,
		InvoiceExemptionReason: c.InvoiceExemptionReason,
		ContactInfo:            contactInfos,
		Notes:                  notes,
		CreatedAt:              c.CreatedAt,
		UpdatedAt:              c.UpdatedAt,
	}
}

func (ctrl *controller) toAPIContactInfo(ci *model.ContactInfo) APIContactInfo {
	return APIContactInfo{
		ID:        ci.ID,
		CreatedAt: ci.CreatedAt,
		UpdatedAt: ci.UpdatedAt,
		Type:      ci.Type,
		Value:     ci.Value,
		Label:     ci.Label,
	}
}

/*
AuthorID   uint      `json:"author_id" xml:"author_id"`
ParentID   uint      `json:"parent_id" xml:"parent_id"`
ParentType string    `json:"parent_type" xml:"parent_type"`
Title      string    `json:"title" xml:"title"`
Body       string    `json:"body" xml:"body"`
Tags       string    `json:"tags" xml:"tags"`
EditedAt   time.Time `json:"edited_at" xml:"edited_at"`
*/
func (ctrl *controller) toAPINote(n *model.Note) APINote {
	return APINote{
		ID:         n.ID,
		CreatedAt:  n.CreatedAt,
		UpdatedAt:  n.UpdatedAt,
		AuthorID:   n.AuthorID,
		ParentID:   n.ParentID,
		ParentType: string(n.ParentType),
		Title:      n.Title,
		Body:       n.Body,
		Tags:       n.Tags,
		EditedAt:   n.EditedAt,
	}
}

func (ctrl *controller) exportCustomersXML(
	ctx context.Context,
	zw *zip.Writer,
	ownerID uint,
) error {
	companies, err := ctrl.model.ListCompaniesForExportCtx(ctx, ownerID)
	if err != nil {
		return fmt.Errorf("cannot load customers for export: %w", err)
	}

	f, err := zw.Create("customers.xml")
	if err != nil {
		return fmt.Errorf("cannot create customers.xml in ZIP: %w", err)
	}

	export := ExportCustomers{
		Version:   "1",
		Customers: make([]APICustomer, 0, len(companies)),
	}

	for i := range companies {
		export.Customers = append(export.Customers, ctrl.toAPICustomer(&companies[i]))
	}

	enc := xml.NewEncoder(f)
	enc.Indent("", "  ")
	if err := enc.Encode(export); err != nil {
		return fmt.Errorf("cannot encode customers.xml: %w", err)
	}
	if err := enc.Flush(); err != nil {
		return fmt.Errorf("cannot flush customers.xml: %w", err)
	}

	return nil
}

func (ctrl *controller) toAPIPerson(p *model.Person) APIPerson {
	contactInfos := make([]APIContactInfo, len(p.ContactInfos))
	for i := range p.ContactInfos {
		contactInfos[i] = ctrl.toAPIContactInfo(&p.ContactInfos[i])
	}

	notes := make([]APINote, len(p.Notes))
	for i := range p.Notes {
		notes[i] = ctrl.toAPINote(&p.Notes[i])
	}

	return APIPerson{
		ID:           p.ID,
		Name:         p.Name,
		Position:     p.Position,
		Email:        p.EMail,
		CompanyID:    p.CompanyID,
		ContactInfos: contactInfos,
		Notes:        notes,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
	}
}

func (ctrl *controller) exportPersonsXML(
	ctx context.Context,
	zw *zip.Writer,
	ownerID uint,
) error {
	persons, err := ctrl.model.ListPersonsForExportCtx(ctx, ownerID)
	if err != nil {
		return fmt.Errorf("cannot load persons for export: %w", err)
	}

	f, err := zw.Create("persons.xml")
	if err != nil {
		return fmt.Errorf("cannot create persons.xml in ZIP: %w", err)
	}

	export := ExportPersons{
		Version: "1",
		Persons: make([]APIPerson, 0, len(persons)),
	}

	for i := range persons {
		export.Persons = append(export.Persons, ctrl.toAPIPerson(&persons[i]))
	}

	enc := xml.NewEncoder(f)
	enc.Indent("", "  ")
	if err := enc.Encode(export); err != nil {
		return fmt.Errorf("cannot encode persons.xml: %w", err)
	}
	if err := enc.Flush(); err != nil {
		return fmt.Errorf("cannot flush persons.xml: %w", err)
	}

	return nil
}

func (ctrl *controller) toAPISettings(s *model.Settings) APISettings {
	return APISettings{
		CompanyName:           s.CompanyName,
		InvoiceContact:        s.InvoiceContact,
		InvoiceEMail:          s.InvoiceEMail,
		ZIP:                   s.ZIP,
		Address1:              s.Address1,
		Address2:              s.Address2,
		City:                  s.City,
		CountryCode:           s.CountryCode,
		VATID:                 s.VATID,
		TAXNumber:             s.TAXNumber,
		InvoiceNumberTemplate: s.InvoiceNumberTemplate,
		UseLocalCounter:       s.UseLocalCounter,
		BankIBAN:              s.BankIBAN,
		BankName:              s.BankName,
		BankBIC:               s.BankBIC,
		CustomerNumberPrefix:  s.CustomerNumberPrefix,
		CustomerNumberWidth:   s.CustomerNumberWidth,
		CustomerNumberCounter: s.CustomerNumberCounter,
	}
}

func (ctrl *controller) exportSettingsXML(
	ctx context.Context,
	zw *zip.Writer,
	ownerID uint,
) error {
	s, err := ctrl.model.LoadSettingsForExportCtx(ctx, ownerID)
	if err != nil {
		// Wenn es wirklich Fälle ohne Settings gibt, könntest du hier auch "kein settings.xml" zulassen.
		return fmt.Errorf("cannot load settings for export: %w", err)
	}

	f, err := zw.Create("settings.xml")
	if err != nil {
		return fmt.Errorf("cannot create settings.xml in ZIP: %w", err)
	}

	export := ExportSettings{
		Version: "1",
		Setting: ctrl.toAPISettings(s),
	}

	enc := xml.NewEncoder(f)
	enc.Indent("", "  ")
	if err := enc.Encode(export); err != nil {
		return fmt.Errorf("cannot encode settings.xml: %w", err)
	}
	return enc.Flush()
}

func (ctrl *controller) toAPILetterheadTemplate(t *model.LetterheadTemplate) APILetterheadTemplate {
	regions := make([]APILetterheadRegion, len(t.Regions))
	for i := range t.Regions {
		regions[i] = ctrl.toAPILetterheadRegion(&t.Regions[i])
	}

	return APILetterheadTemplate{
		ID:              t.ID,
		Name:            t.Name,
		PageWidthCm:     t.PageWidthCm,
		PageHeightCm:    t.PageHeightCm,
		PDFPath:         t.PDFPath,
		PreviewPage1URL: t.PreviewPage1URL,
		PreviewPage2URL: t.PreviewPage2URL,
		FontNormal:      t.FontNormal,
		FontBold:        t.FontBold,
		FontItalic:      t.FontItalic,
		Regions:         regions,
		CreatedAt:       t.CreatedAt,
		UpdatedAt:       t.UpdatedAt,
	}
}

func (ctrl *controller) toAPILetterheadRegion(r *model.PlacedRegion) APILetterheadRegion {
	return APILetterheadRegion{
		ID:          r.ID,
		Kind:        string(r.Kind),
		Page:        r.Page,
		XCm:         r.XCm,
		YCm:         r.YCm,
		WidthCm:     r.WidthCm,
		HeightCm:    r.HeightCm,
		HAlign:      r.HAlign,
		VAlign:      r.VAlign,
		FontName:    r.FontName,
		FontSizePt:  r.FontSizePt,
		LineSpacing: r.LineSpacing,
		HasPage2:    r.HasPage2,
		X2Cm:        r.X2Cm,
		Y2Cm:        r.Y2Cm,
		Width2Cm:    r.Width2Cm,
		Height2Cm:   r.Height2Cm,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

func (ctrl *controller) exportLetterheadTemplatesXML(
	ctx context.Context,
	zw *zip.Writer,
	ownerID uint,
) error {
	templates, err := ctrl.model.ListLetterheadTemplatesForExportCtx(ctx, ownerID)
	if err != nil {
		return fmt.Errorf("cannot load letterhead templates for export: %w", err)
	}

	f, err := zw.Create("letterhead_templates.xml")
	if err != nil {
		return fmt.Errorf("cannot create letterhead_templates.xml in ZIP: %w", err)
	}

	export := ExportLetterheadTemplates{
		Version:   "1",
		Templates: make([]APILetterheadTemplate, 0, len(templates)),
	}

	for i := range templates {
		export.Templates = append(export.Templates, ctrl.toAPILetterheadTemplate(&templates[i]))
	}

	enc := xml.NewEncoder(f)
	enc.Indent("", "  ")
	if err := enc.Encode(export); err != nil {
		return fmt.Errorf("cannot encode letterhead_templates.xml: %w", err)
	}
	if err := enc.Flush(); err != nil {
		return fmt.Errorf("cannot flush letterhead_templates.xml: %w", err)
	}

	return nil
}

// addFileToZip copies a single file from disk into the ZIP archive
// under the given zipPath.
func (ctrl *controller) addFileToZip(zw *zip.Writer, srcPath, zipPath string) error {
	f, err := os.Open(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	w, err := zw.Create(zipPath)
	if err != nil {
		return err
	}

	if _, err := io.Copy(w, f); err != nil {
		return err
	}
	return nil
}

// exportUserAssets adds all user-uploaded assets for this owner into the ZIP.
// Base directory is: Basedir/assets/userassets/owner{ownerID}
// Files werden im ZIP unter assets/userassets/owner{ownerID}/... abgelegt.
func (ctrl *controller) exportUserAssets(zw *zip.Writer, ownerID uint) error {
	baseDir := filepath.Join(
		ctrl.model.Config.Basedir,
		"assets",
		"userassets",
		fmt.Sprintf("owner%d", ownerID),
	)

	// if baseDir does not exist or is not a directory, skip
	fi, err := os.Stat(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat user assets dir: %w", err)
	}
	if !fi.IsDir() {
		return nil
	}

	// walk through the directory and add all files to the ZIP
	err = filepath.WalkDir(baseDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(baseDir, path)
		if err != nil {
			return err
		}

		// path in ZIP: assets/userassets/owner{ownerID}/<relative path>
		zipPath := filepath.ToSlash(filepath.Join(
			"assets",
			"userassets",
			fmt.Sprintf("owner%d", ownerID),
			rel,
		))

		if err := ctrl.addFileToZip(zw, path, zipPath); err != nil {
			return fmt.Errorf("add user asset %q: %w", path, err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk user assets dir: %w", err)
	}

	return nil
}

// exportInvoiceFiles adds all invoice-related PDF and XML files for this owner
// into the ZIP.
//
// Base directory: XMLDir/owner{ownerID}
// - all *.pdf → z.B.
// - Alle *.xml mit numerischem Dateinamen (1234.xml) → invoices/xml/
func (ctrl *controller) exportInvoiceFiles(zw *zip.Writer, ownerID uint) error {
	baseDir := filepath.Join(
		ctrl.model.Config.XMLDir,
		fmt.Sprintf("owner%d", ownerID),
	)

	fi, err := os.Stat(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			// no directory → nothing to do
			return nil
		}
		return fmt.Errorf("stat invoice files dir: %w", err)
	}
	if !fi.IsDir() {
		// no directory → nothing to do
		return nil
	}

	// get all PDF files in the directory
	pdfGlob := filepath.Join(baseDir, "*.pdf")
	pdfFiles, err := filepath.Glob(pdfGlob)
	if err != nil {
		return fmt.Errorf("glob pdf files: %w", err)
	}

	for _, pdfPath := range pdfFiles {
		name := filepath.Base(pdfPath)           // e.g. "1234.pdf"
		base := strings.TrimSuffix(name, ".pdf") // "1234"

		if base == "" {
			continue
		}

		// optional: only consider numeric basenames
		numeric := true
		for _, r := range base {
			if r < '0' || r > '9' {
				numeric = false
				break
			}
		}
		if !numeric {
			continue
		}

		// 1) put PDF into ZIP
		pdfZipPath := filepath.ToSlash(filepath.Join("invoices", "pdf", name))
		if err := ctrl.addFileToZip(zw, pdfPath, pdfZipPath); err != nil {
			return fmt.Errorf("add pdf %q: %w", pdfPath, err)
		}

		// 2) try matching xml: <id>.xml
		xmlName := base + ".xml"
		xmlPath := filepath.Join(baseDir, xmlName)
		if _, err := os.Stat(xmlPath); err != nil {
			if os.IsNotExist(err) {
				// no matching XML → skip
				continue
			}
			return fmt.Errorf("stat xml %q: %w", xmlPath, err)
		}

		xmlZipPath := filepath.ToSlash(filepath.Join("invoices", "xml", xmlName))
		if err := ctrl.addFileToZip(zw, xmlPath, xmlZipPath); err != nil {
			return fmt.Errorf("add xml %q: %w", xmlPath, err)
		}
	}

	return nil
}
