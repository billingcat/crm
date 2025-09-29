package controller

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/billingcat/crm/model"
	"github.com/go-playground/form/v4"
	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

func (ctrl *controller) companyInit(e *echo.Echo) {
	g := e.Group("/company")
	g.Use(ctrl.authMiddleware)
	g.GET("/new", ctrl.companynew)
	g.POST("/new", ctrl.companynew)
	g.GET("/edit/:id", ctrl.companyedit)
	g.POST("/edit/:id", ctrl.companyedit)
	g.GET("/:id/:name", ctrl.companydetail)
	g.GET("/:id", ctrl.companydetail)
}

// ---- Form-Types ----

type contactInfoForm struct {
	Type  string `form:"type"`  // phone | fax | email | website | linkedin | twitter | github | other
	Label string `form:"label"` // Bezeichnung (z.B. Büro, Support)
	Value string `form:"value"` // eigentliche Nummer/URL/E-Mail
}

type companyForm struct {
	Background             string            `form:"background"`
	Name                   string            `form:"name"`
	Kundennummer           string            `form:"kundennummer"`
	RechnungEmail          string            `form:"rechnungemail"`
	SupplierNumber         string            `form:"suppliernumber"`
	ContactInvoice         string            `form:"contactinvoice"`
	DefaultTaxRate         string            `form:"defaulttaxrate"`
	Adresse1               string            `form:"adresse1"`
	Adresse2               string            `form:"adresse2"`
	PLZ                    string            `form:"plz"`
	Ort                    string            `form:"ort"`
	Phone                  []contactInfoForm `form:"phone"` // <- bleibt "phone", damit das Template passt
	Land                   string            `form:"land"`
	VATID                  string            `form:"vatid"`
	InvoiceOpening         string            `form:"invoiceopening"`
	InvoiceCurrency        string            `form:"invoicecurrency"`
	InvoiceTaxType         string            `form:"invoicetaxtype"`
	InvoiceFooter          string            `form:"invoicefooter"`
	InvoiceExemptionReason string            `form:"invoiceexemptionreason"`
}

func (ctrl *controller) companynew(c echo.Context) error {
	m := ctrl.defaultResponseMap(c, "Neue Firma anlegen")
	switch c.Request().Method {
	case http.MethodGet:
		m["submit"] = "Firma anlegen"
		m["action"] = "/company/new"
		m["cancel"] = "/"
		m["company"] = model.Company{}
		return c.Render(http.StatusOK, "companyedit.html", m)

	case http.MethodPost:
		ownerID := c.Get("ownerid").(uint)

		// Form dekodieren
		if err := c.Request().ParseForm(); err != nil {
			return ErrInvalid(err, "Fehler beim Verarbeiten der Formulardaten")
		}
		var comp companyForm
		dec := form.NewDecoder()
		if err := dec.Decode(&comp, c.Request().Form); err != nil {
			return ErrInvalid(err, "Fehler beim Verarbeiten der Formulardaten")
		}

		dbCompany := &model.Company{
			Adresse1:               comp.Adresse1,
			Adresse2:               comp.Adresse2,
			Background:             comp.Background,
			ContactInvoice:         comp.ContactInvoice,
			InvoiceCurrency:        comp.InvoiceCurrency,
			InvoiceExemptionReason: comp.InvoiceExemptionReason,
			InvoiceFooter:          comp.InvoiceFooter,
			InvoiceOpening:         comp.InvoiceOpening,
			InvoiceTaxType:         comp.InvoiceTaxType,
			Kundennummer:           comp.Kundennummer,
			Land:                   comp.Land,
			Name:                   comp.Name,
			Ort:                    comp.Ort,
			OwnerID:                ownerID,
			PLZ:                    comp.PLZ,
			RechnungEmail:          comp.RechnungEmail,
			SupplierNumber:         comp.SupplierNumber,
			VATID:                  comp.VATID,
		}

		// Kontaktinfos übernehmen (nur nicht-leere Value)
		for _, ci := range comp.Phone {
			ci.Type = strings.TrimSpace(ci.Type)
			ci.Label = strings.TrimSpace(ci.Label)
			ci.Value = strings.TrimSpace(ci.Value)
			if ci.Value == "" {
				continue
			}
			dbCI := model.ContactInfo{
				Type:       ci.Type,
				Label:      ci.Label,
				Value:      ci.Value,
				OwnerID:    ownerID,
				ParentType: "company", // wichtig bei Polymorphie
				// ParentID wird von GORM beim Assoc-Save gesetzt
			}
			dbCompany.ContactInfos = append(dbCompany.ContactInfos, dbCI)
		}

		var err error
		dbCompany.DefaultTaxRate, err = decimal.NewFromString(strings.TrimSpace(comp.DefaultTaxRate))
		if err != nil {
			return ErrInvalid(err, "Fehler beim Verarbeiten der Mehrwertsteuer")
		}

		if err := ctrl.model.SaveCompany(dbCompany, ownerID); err != nil {
			return ErrInvalid(err, "Fehler beim Speichern der Firma")
		}
		return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/company/%d", dbCompany.ID))
	}
	return fmt.Errorf("Unknown method %s", c.Request().Method)
}

func (ctrl *controller) companydetail(c echo.Context) error {
	m := ctrl.defaultResponseMap(c, "Kontakt Details")
	ownerID := c.Get("ownerid").(uint)
	companyDB, err := ctrl.model.LoadCompany(c.Param("id"), ownerID)
	if err != nil {
		return ErrInvalid(err, "Kann Firma nicht laden")
	}
	notes, err := ctrl.model.LoadAllNotesForParent(ownerID, "companies", companyDB.ID)
	if err != nil {
		return ErrInvalid(err, "Kann Notizen nicht laden")
	}
	m["notes"] = notes
	m["right"] = "companydetail"
	m["companydetail"] = companyDB
	m["title"] = companyDB.Name
	ctrl.model.TouchRecentView(ownerID, model.EntityCompany, companyDB.ID)
	return c.Render(http.StatusOK, "companydetail.html", m)
}

func (ctrl *controller) companyedit(c echo.Context) error {
	m := ctrl.defaultResponseMap(c, "Firma bearbeiten")
	ownerID := c.Get("ownerid").(uint)

	switch c.Request().Method {
	case http.MethodGet:
		paramCompanyID := c.Param("id")
		company, err := ctrl.model.LoadCompany(paramCompanyID, ownerID)
		if err != nil {
			return ErrInvalid(fmt.Errorf("cannot find company with id %v and ownerid %v", paramCompanyID, ownerID), "Kann Firma nicht laden")
		}

		m["title"] = company.Name + " bearbeiten"
		m["company"] = company
		m["action"] = fmt.Sprintf("/company/edit/%d", company.ID)
		m["cancel"] = fmt.Sprintf("/company/%d", company.ID)
		m["submit"] = "Daten ändern"
		return c.Render(http.StatusOK, "companyedit.html", m)

	case http.MethodPost:
		// Form dekodieren
		if err := c.Request().ParseForm(); err != nil {
			return ErrInvalid(err, "Fehler beim Verarbeiten der Formulardaten")
		}
		var comp companyForm
		dec := form.NewDecoder()
		if err := dec.Decode(&comp, c.Request().Form); err != nil {
			return ErrInvalid(err, "Fehler beim Verarbeiten der Formulardaten")
		}

		paramCompanyID := c.Param("id")
		dbCompany, err := ctrl.model.LoadCompany(paramCompanyID, ownerID)
		if err != nil {
			return ErrInvalid(fmt.Errorf("cannot find company with id %v and ownerid %v", paramCompanyID, ownerID), "Kann Firma nicht laden")
		}

		// Stammdaten aktualisieren
		dbCompany.Background = comp.Background
		dbCompany.Name = comp.Name
		dbCompany.Kundennummer = comp.Kundennummer
		dbCompany.Adresse1 = comp.Adresse1
		dbCompany.Adresse2 = comp.Adresse2
		dbCompany.RechnungEmail = comp.RechnungEmail
		dbCompany.SupplierNumber = comp.SupplierNumber
		dbCompany.ContactInvoice = comp.ContactInvoice
		dbCompany.Ort = comp.Ort
		dbCompany.PLZ = comp.PLZ
		dbCompany.VATID = comp.VATID
		dbCompany.Land = comp.Land
		dbCompany.InvoiceOpening = comp.InvoiceOpening
		dbCompany.InvoiceCurrency = comp.InvoiceCurrency
		dbCompany.InvoiceTaxType = comp.InvoiceTaxType
		dbCompany.InvoiceFooter = comp.InvoiceFooter
		dbCompany.InvoiceExemptionReason = comp.InvoiceExemptionReason

		if dbCompany.DefaultTaxRate, err = decimal.NewFromString(strings.TrimSpace(comp.DefaultTaxRate)); err != nil {
			return ErrInvalid(err, "Fehler beim Verarbeiten der Mehrwertsteuer")
		}

		// Kontaktinfos neu setzen (einfachste Variante: ersetzen)
		dbCompany.ContactInfos = []model.ContactInfo{}
		for _, ci := range comp.Phone {
			ci.Type = strings.TrimSpace(ci.Type)
			ci.Label = strings.TrimSpace(ci.Label)
			ci.Value = strings.TrimSpace(ci.Value)
			if ci.Value == "" {
				continue
			}
			dbCI := model.ContactInfo{
				Type:       ci.Type,
				Label:      ci.Label,
				Value:      ci.Value,
				OwnerID:    ownerID,
				ParentType: "company",
			}
			dbCompany.ContactInfos = append(dbCompany.ContactInfos, dbCI)
		}

		if err := ctrl.model.SaveCompany(dbCompany, ownerID); err != nil {
			return ErrInvalid(err, "Fehler beim Speichern der Firma")
		}
		if err = c.Redirect(http.StatusSeeOther, fmt.Sprintf("/company/%d/%s", dbCompany.ID, dbCompany.Name)); err != nil {
			return ErrInvalid(err, "Fehler beim Weiterleiten zur Firmenseite")
		}
	}
	return nil
}
