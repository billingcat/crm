package controller

import (
	"fmt"
	"net/http"

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

type phoneForm struct {
	Number   string `form:"number"`
	Location string `form:"location"`
}

type company struct {
	Background             string      `form:"background"`
	Name                   string      `form:"name"`
	Kundennummer           string      `form:"kundennummer"`
	RechnungEmail          string      `form:"rechnungemail"`
	SupplierNumber         string      `form:"suppliernumber"`
	ContactInvoice         string      `form:"contactinvoice"`
	DefaultTaxRate         string      `form:"defaulttaxrate"`
	Adresse1               string      `form:"adresse1"`
	Adresse2               string      `form:"adresse2"`
	PLZ                    string      `form:"plz"`
	Ort                    string      `form:"ort"`
	Phone                  []phoneForm `form:"phone"`
	Land                   string      `form:"land"`
	VATID                  string      `form:"vatid"`
	InvoiceOpening         string      `form:"invoiceopening"`
	InvoiceCurrency        string      `form:"invoicecurrency"`
	InvoiceTaxType         string      `form:"invoicetaxtype"`
	InvoiceFooter          string      `form:"invoicefooter"`
	InvoiceExemptionReason string      `form:"invoiceexemptionreason"`
}

type phone struct {
	Number   string
	Location string
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
		var comp company
		var err error
		err = c.Request().ParseForm()
		if err != nil {
			return ErrInvalid(err, "Fehler beim Verarbeiten der Formulardaten")
		}
		dec := form.NewDecoder()
		err = dec.Decode(&c, c.Request().Form)
		if err != nil {
			return ErrInvalid(err, "Fehler beim Verarbeiten der Formulardaten")
		}

		if err := c.Bind(&comp); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "bad request")
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
		for _, ph := range comp.Phone {
			dbPhone := model.Phone{
				Number:     ph.Number,
				Location:   ph.Location,
				OwnerID:    ownerID,
				ParentID:   int(dbCompany.ID),
				ParentType: "company",
			}
			dbCompany.Phones = append(dbCompany.Phones, dbPhone)
		}
		dbCompany.DefaultTaxRate, err = decimal.NewFromString(comp.DefaultTaxRate)
		if err != nil {
			return ErrInvalid(err, "Fehler beim Verarbeiten der Mehrwertsteuer")
		}

		if err := ctrl.model.SaveCompany(dbCompany, dbCompany.OwnerID); err != nil {
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
		var comp company
		var err error
		err = c.Request().ParseForm()
		if err != nil {
			return ErrInvalid(err, "Fehler beim Verarbeiten der Formulardaten")
		}
		dec := form.NewDecoder()
		err = dec.Decode(&comp, c.Request().Form)
		if err != nil {
			return ErrInvalid(err, "Fehler beim Verarbeiten der Formulardaten")
		}
		paramCompanyID := c.Param("id")
		dbCompany, err := ctrl.model.LoadCompany(paramCompanyID, ownerID)
		if err != nil {
			return ErrInvalid(fmt.Errorf("cannot find company with id %v and ownerid %v", paramCompanyID, ownerID), "Kann Firma nicht laden")
		}
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
		dbCompany.DefaultTaxRate, err = decimal.NewFromString(comp.DefaultTaxRate)
		dbCompany.Phones = []model.Phone{}
		if err != nil {
			return ErrInvalid(err, "Fehler beim Verarbeiten der Mehrwertsteuer")
		}
		for _, ph := range comp.Phone {
			dbPhone := model.Phone{
				Number:     ph.Number,
				Location:   ph.Location,
				OwnerID:    ownerID,
				ParentID:   int(dbCompany.ID),
				ParentType: "company",
			}
			dbCompany.Phones = append(dbCompany.Phones, dbPhone)
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
