package fixtures

import (
	"testing"

	"github.com/billingcat/crm/model"
)

// SeedLetterheadTemplate creates and persists a letterhead template with the
// three standard regions (addressee, invoice_info, main_area) for the default
// owner. pdfRelPath is stored as the template's PDFPath (relative to the owner's
// asset directory); pass "" for a template without a background PDF.
func SeedLetterheadTemplate(t *testing.T, store *model.Store, pdfRelPath string) *model.LetterheadTemplate {
	t.Helper()
	tpl := &model.LetterheadTemplate{
		OwnerID:      DefaultOwnerID,
		Name:         "Musterbriefbogen",
		PageWidthCm:  21.0,
		PageHeightCm: 29.7,
		PDFPath:      pdfRelPath,
		Regions: []model.PlacedRegion{
			{
				OwnerID: DefaultOwnerID, Kind: model.FieldSender, Page: 1,
				XCm: 2.0, YCm: 5.2, WidthCm: 8.5, HeightCm: 3.0,
				HAlign: "left", FontSizePt: 10, LineSpacing: 1.2,
			},
			{
				OwnerID: DefaultOwnerID, Kind: model.FieldInvoiceInfo, Page: 1,
				XCm: 12.0, YCm: 5.0, WidthCm: 7.0, HeightCm: 4.0,
				HAlign: "right", FontSizePt: 10, LineSpacing: 1.2,
			},
			{
				OwnerID: DefaultOwnerID, Kind: model.FieldPositions, Page: 1,
				XCm: 2.0, YCm: 10.0, WidthCm: 17.0, HeightCm: 15.0,
				HAlign: "left", FontSizePt: 10, LineSpacing: 1.2,
			},
		},
	}
	if err := store.SaveLetterheadTemplate(tpl, DefaultOwnerID); err != nil {
		t.Fatalf("seed letterhead template: %v", err)
	}
	return tpl
}

// AttachTemplateToInvoice sets inv's TemplateID and persists the change, so that
// LoadInvoiceWithTemplate then preloads the template and its regions.
func AttachTemplateToInvoice(t *testing.T, store *model.Store, inv *model.Invoice, templateID uint) {
	t.Helper()
	inv.TemplateID = &templateID
	if err := store.UpdateInvoice(inv, DefaultOwnerID); err != nil {
		t.Fatalf("attach template to invoice: %v", err)
	}
}
