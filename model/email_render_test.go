package model_test

import (
	"strings"
	"testing"

	"github.com/billingcat/crm/fixtures"
	"github.com/billingcat/crm/model"
)

func TestRenderInvoiceMail_HardcodedDefault(t *testing.T) {
	store := fixtures.NewTestStore(t)
	data := fixtures.SeedTestData(t, store)

	subject, body, err := store.RenderInvoiceMail(fixtures.DefaultOwnerID, data.Invoice, data.Company)
	if err != nil {
		t.Fatalf("RenderInvoiceMail failed: %v", err)
	}
	if !strings.Contains(subject, data.Invoice.Number) {
		t.Errorf("subject %q should contain invoice number %q", subject, data.Invoice.Number)
	}
	if !strings.Contains(body, data.Invoice.Number) {
		t.Errorf("body should contain invoice number, got:\n%s", body)
	}
	if !strings.Contains(body, "Sehr geehrte") {
		t.Errorf("body should contain hardcoded greeting, got:\n%s", body)
	}
}

func TestRenderInvoiceMail_OwnerOverride(t *testing.T) {
	store := fixtures.NewTestStore(t)
	data := fixtures.SeedTestData(t, store)

	if err := store.SaveEmailTemplate(&model.EmailTemplate{
		OwnerID: fixtures.DefaultOwnerID,
		Kind:    model.EmailTemplateKindInvoice,
		Subject: "Globaler Betreff {{.Number}}",
		Body:    "Globaler Body für {{.Company}}",
	}); err != nil {
		t.Fatalf("SaveEmailTemplate: %v", err)
	}

	subject, body, err := store.RenderInvoiceMail(fixtures.DefaultOwnerID, data.Invoice, data.Company)
	if err != nil {
		t.Fatalf("RenderInvoiceMail: %v", err)
	}
	if want := "Globaler Betreff " + data.Invoice.Number; subject != want {
		t.Errorf("subject = %q, want %q", subject, want)
	}
	if want := "Globaler Body für " + data.Company.Name; body != want {
		t.Errorf("body = %q, want %q", body, want)
	}
}

func TestRenderInvoiceMail_CompanyOverride(t *testing.T) {
	store := fixtures.NewTestStore(t)
	data := fixtures.SeedTestData(t, store)

	// Global default
	_ = store.SaveEmailTemplate(&model.EmailTemplate{
		OwnerID: fixtures.DefaultOwnerID,
		Kind:    model.EmailTemplateKindInvoice,
		Subject: "Global",
		Body:    "Global",
	})
	// Company-specific override
	if err := store.SaveEmailTemplate(&model.EmailTemplate{
		OwnerID:   fixtures.DefaultOwnerID,
		CompanyID: data.Company.ID,
		Kind:      model.EmailTemplateKindInvoice,
		Subject:   "Spezial",
		Body:      "Spezial",
	}); err != nil {
		t.Fatalf("SaveEmailTemplate company: %v", err)
	}

	subject, body, err := store.RenderInvoiceMail(fixtures.DefaultOwnerID, data.Invoice, data.Company)
	if err != nil {
		t.Fatalf("RenderInvoiceMail: %v", err)
	}
	if subject != "Spezial" {
		t.Errorf("subject = %q, want %q", subject, "Spezial")
	}
	if body != "Spezial" {
		t.Errorf("body = %q, want %q", body, "Spezial")
	}
}

func TestRenderInvoiceMail_PartialCompanyOverride(t *testing.T) {
	// Company overrides only subject; body should fall back to owner default.
	store := fixtures.NewTestStore(t)
	data := fixtures.SeedTestData(t, store)

	_ = store.SaveEmailTemplate(&model.EmailTemplate{
		OwnerID: fixtures.DefaultOwnerID,
		Kind:    model.EmailTemplateKindInvoice,
		Subject: "Global",
		Body:    "Global Body",
	})
	_ = store.SaveEmailTemplate(&model.EmailTemplate{
		OwnerID:   fixtures.DefaultOwnerID,
		CompanyID: data.Company.ID,
		Kind:      model.EmailTemplateKindInvoice,
		Subject:   "Nur-Spezial-Subject",
		// Body left empty -> use owner default
	})

	subject, body, err := store.RenderInvoiceMail(fixtures.DefaultOwnerID, data.Invoice, data.Company)
	if err != nil {
		t.Fatalf("RenderInvoiceMail: %v", err)
	}
	if subject != "Nur-Spezial-Subject" {
		t.Errorf("subject = %q, want %q", subject, "Nur-Spezial-Subject")
	}
	if body != "Global Body" {
		t.Errorf("body = %q, want %q", body, "Global Body")
	}
}

func TestRenderInvoiceMail_InvalidTemplateFallsBack(t *testing.T) {
	// A broken template should not break the mailto link — render falls back.
	store := fixtures.NewTestStore(t)
	data := fixtures.SeedTestData(t, store)

	_ = store.SaveEmailTemplate(&model.EmailTemplate{
		OwnerID: fixtures.DefaultOwnerID,
		Kind:    model.EmailTemplateKindInvoice,
		Subject: "{{.MissingFunc | nope}}", // parse error
		Body:    "ok",
	})

	subject, body, err := store.RenderInvoiceMail(fixtures.DefaultOwnerID, data.Invoice, data.Company)
	if err != nil {
		t.Fatalf("RenderInvoiceMail: %v", err)
	}
	if !strings.Contains(subject, data.Invoice.Number) {
		t.Errorf("expected fallback subject containing invoice number, got %q", subject)
	}
	if body != "ok" {
		t.Errorf("body = %q, want %q", body, "ok")
	}
}

func TestSaveEmailTemplate_EmptyDeletes(t *testing.T) {
	store := fixtures.NewTestStore(t)
	data := fixtures.SeedTestData(t, store)

	if err := store.SaveEmailTemplate(&model.EmailTemplate{
		OwnerID:   fixtures.DefaultOwnerID,
		CompanyID: data.Company.ID,
		Kind:      model.EmailTemplateKindInvoice,
		Subject:   "x",
		Body:      "y",
	}); err != nil {
		t.Fatalf("save initial: %v", err)
	}

	// Now save with both empty -> should delete the row
	if err := store.SaveEmailTemplate(&model.EmailTemplate{
		OwnerID:   fixtures.DefaultOwnerID,
		CompanyID: data.Company.ID,
		Kind:      model.EmailTemplateKindInvoice,
	}); err != nil {
		t.Fatalf("save empty: %v", err)
	}

	got, err := store.LoadCompanyEmailTemplate(fixtures.DefaultOwnerID, data.Company.ID, model.EmailTemplateKindInvoice)
	if err != nil {
		t.Fatalf("LoadCompanyEmailTemplate: %v", err)
	}
	if got != nil {
		t.Errorf("expected company override to be deleted, got %+v", got)
	}
}
