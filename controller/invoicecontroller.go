package controller

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/billingcat/crm/model"

	"github.com/go-playground/form/v4"
	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

var (
	commaperiod            = strings.NewReplacer(",", ".")
	customerNumberReplacer = regexp.MustCompile(`%CN%`)
	counterReplacer        = regexp.MustCompile(`%(0?)(\d+)C%`)
)

func (ctrl *controller) invoiceInit(e *echo.Echo) {
	g := e.Group("/invoice")
	g.Use(ctrl.authMiddleware)
	g.GET("/new/:companyid", ctrl.invoiceNew)
	g.POST("/new", ctrl.invoiceNew)
	g.GET("/detail/:id", ctrl.invoiceDetail)
	g.DELETE("/delete/:id", ctrl.invoiceDelete)
	g.GET("/duplicate/:id", ctrl.invoiceDuplicate)
	g.GET("/edit/:id", ctrl.invoiceEdit)
	g.POST("/edit/:id", ctrl.invoiceEdit)
	g.GET("/zugferdxml/:id", ctrl.invoiceZUGFeRDXML)
	g.GET("/zugferdpdf/:id", ctrl.invoiceZUGFeRDPDF)
}

// invoicepos has one invoice line
type invoicepos struct {
	Menge         string `form:"menge"`
	Einzelpreis   string `form:"einzelpreis"`
	Gesamtpreis   string `form:"gesamtpreis"`
	Einheit       string `form:"einheit"`
	Leistungstext string `form:"leistungstext"`
	Steuersatz    string `form:"steuersatz"`
}

type invoice struct {
	Anrede                 string       `form:"anrede"`
	CompanyID              uint         `form:"companyid"`
	ContactInvoice         string       `form:"contactinvoice"`
	Counter                uint         `form:"counter"`
	Currency               string       `form:"currency"`
	Date                   time.Time    `form:"date"`
	DueDate                time.Time    `form:"duedate"`
	Empfaenger             string       `form:"empfaenger"`
	Fusszeile              string       `form:"fusszeile"`
	InvoiceExemptionReason string       `form:"invoiceexemptionreason"`
	InvoiceID              uint         `form:"invoiceid"`
	InvoiceNumber          string       `form:"invoicenumber"`
	Invoicepos             []invoicepos `form:"invoicepos"`
	Leistungsdatum         time.Time    `form:"occurrencedate"`
	OrderNumber            string       `form:"ordernumber"`
	SupplierNumber         string       `form:"suppliernumber"`
	Taxtype                string       `form:"taxtype"`
	VATID                  string       `form:"ustid"`
}

func bindInvoice(c echo.Context) (*model.Invoice, error) {
	ownerID := c.Get("ownerid").(uint)
	i := invoice{}
	dec := form.NewDecoder()
	dec.RegisterCustomTypeFunc(func(vals []string) (interface{}, error) {
		return time.Parse("2006-01-02", vals[0])
	}, time.Time{})
	err := c.Request().ParseForm()
	if err != nil {
		return nil, err
	}

	err = dec.Decode(&i, c.Request().Form)
	if err != nil {
		return nil, err
	}
	counter := 0
	mi := &model.Invoice{
		Number:          i.InvoiceNumber,
		Date:            i.Date,
		OccurrenceDate:  i.Leistungsdatum,
		DueDate:         i.DueDate,
		ContactInvoice:  i.ContactInvoice,
		Counter:         i.Counter,
		OrderNumber:     i.OrderNumber,
		SupplierNumber:  i.SupplierNumber,
		Footer:          i.Fusszeile,
		Opening:         i.Anrede,
		TaxType:         i.Taxtype,
		Currency:        i.Currency,
		TaxNumber:       i.VATID,
		CompanyID:       i.CompanyID,
		ExemptionReason: i.InvoiceExemptionReason,
		OwnerID:         ownerID,
	}
	mi.ID = i.InvoiceID

	for _, ip := range i.Invoicepos {
		if ip.Menge != "0" && ip.Menge != "" {
			counter++
			mip := model.InvoicePosition{
				Position: counter,
				UnitCode: ip.Einheit,
				Text:     ip.Leistungstext,
			}
			if mip.NetPrice, err = decimal.NewFromString(commaperiod.Replace(ip.Einzelpreis)); err != nil {
				return nil, err
			}
			mip.GrossPrice = mip.NetPrice.Copy()
			if mip.Quantity, err = decimal.NewFromString(commaperiod.Replace(ip.Menge)); err != nil {
				return nil, err
			}
			if mip.TaxRate, err = decimal.NewFromString(commaperiod.Replace(ip.Steuersatz)); err != nil {
				return nil, err
			}
			if mip.LineTotal, err = decimal.NewFromString(commaperiod.Replace(ip.Gesamtpreis)); err != nil {
				return nil, err
			}
			mip.OwnerID = ownerID
			mi.InvoicePositions = append(mi.InvoicePositions, mip)
		}
	}
	return mi, nil
}

func formatInvoiceNumber(in string, customernumber string, counter int) string {
	in = customerNumberReplacer.ReplaceAllLiteralString(in, customernumber)
	if counterReplacer.MatchString(in) {
		var formatString string
		x := counterReplacer.FindAllStringSubmatch(in, -1)
		if x[0][1] == "0" {
			formatString = "%0" + x[0][2] + "d"
		} else {
			formatString = "%d"
		}
		in = counterReplacer.ReplaceAllString(in, fmt.Sprintf(formatString, counter))
	}
	return in
}

func (ctrl *controller) invoiceNew(c echo.Context) error {
	m := ctrl.defaultResponseMap(c, "Neue Rechnung anlegen")
	ownerID := c.Get("ownerid").(uint)
	switch c.Request().Method {
	case http.MethodGet:
		s, err := ctrl.model.LoadSettings(ownerID)
		if err != nil {
			return ErrInvalid(err, "Fehler beim Laden der Einstellungen")
		}

		companyID := c.Param("companyid")
		company, err := ctrl.model.LoadCompany(companyID, ownerID)
		if err != nil {
			return ErrInvalid(fmt.Errorf("cannot find company with id %v and ownerid %v", companyID, ownerID), "Kann Firma nicht laden")
		}

		counter, err := ctrl.model.GetMaxCounter(company.ID, s.UseLocalCounter, ownerID)
		if err != nil {
			return nil
		}

		inv := model.Invoice{
			Counter:          counter + 1,
			Date:             time.Now(),
			OccurrenceDate:   time.Now(),
			DueDate:          time.Now().Add(time.Hour * 24 * 14),
			SupplierNumber:   company.SupplierNumber,
			ContactInvoice:   company.ContactInvoice,
			Opening:          company.InvoiceOpening,
			Footer:           company.InvoiceFooter,
			InvoicePositions: []model.InvoicePosition{{Position: 1, TaxRate: company.DefaultTaxRate}},
			Number:           formatInvoiceNumber(s.InvoiceNumberTemplate, company.Kundennummer, int(counter+1)),
			ExemptionReason:  company.InvoiceExemptionReason,
		}
		m["title"] = "Neue Rechnung anlegen"
		m["invoice"] = inv
		m["company"] = company
		m["submit"] = "Rechnung erstellen"
		m["action"] = "/invoice/new"
		m["cancel"] = fmt.Sprintf("/company/%s", companyID)

		return c.Render(http.StatusOK, "invoiceedit.html", m)

	case http.MethodPost:
		mi, err := bindInvoice(c)
		if err != nil {
			return ErrInvalid(err, "Fehler beim Verarbeiten der Eingabedaten")
		}

		if err = ctrl.model.SaveInvoice(mi, ownerID); err != nil {
			return ErrInvalid(err, "Fehler beim Speichern der Rechnung")
		}
		return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/invoice/detail/%d", mi.ID))
	}

	return nil
}

func (ctrl *controller) invoiceDelete(c echo.Context) error {
	paramInvoiceID := c.Param("id")
	ownerID := c.Get("ownerid").(uint)
	inv, err := ctrl.model.LoadInvoice(paramInvoiceID, ownerID)
	if err != nil {
		return ErrInvalid(err, "Kann Rechnung nicht laden")
	}
	if inv.OwnerID != ownerID {
		return echo.NewHTTPError(http.StatusForbidden, "You do not have permission to delete this invoice")
	}
	companyid := inv.CompanyID
	err = ctrl.model.DeleteInvoice(inv, ownerID)
	if err != nil {
		return ErrInvalid(err, "Kann Rechnung nicht löschen")
	}
	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/company/%d", companyid))
}

func (ctrl *controller) invoiceDetail(c echo.Context) error {
	m := ctrl.defaultResponseMap(c, "Rechnung-Details")
	ownerID := c.Get("ownerid").(uint)
	i, err := ctrl.model.LoadInvoice(c.Param("id"), ownerID)
	if err != nil {
		return ErrInvalid(err, "Kann Rechnung nicht laden")
	}
	var cpy *model.Company
	if cpy, err = ctrl.model.LoadCompany(i.CompanyID, ownerID); err != nil {
		return ErrInvalid(err, "Kann Firma nicht laden")
	}
	m["title"] = "Rechnung " + i.Number
	m["invoice"] = i
	m["company"] = cpy
	return c.Render(http.StatusOK, "invoicedetail.html", m)
}

// duplicate an invoice
func (ctrl *controller) invoiceDuplicate(c echo.Context) error {
	m := ctrl.defaultResponseMap(c, "Rechnung duplizieren")
	ownerID := c.Get("ownerid").(uint)
	i, err := ctrl.model.LoadInvoice(c.Param("id"), ownerID)
	if err != nil {
		return ErrInvalid(err, "Kann Rechnung nicht laden")
	}
	if i.OwnerID != ownerID {
		return echo.NewHTTPError(http.StatusForbidden, "You do not have permission to duplicate this invoice")
	}

	// TODO: Create a new invoice based on the existing one
	// Set ID to 0, update date to today, update counter and number
	i.ID = 0
	i.Date = time.Now()
	i.DueDate = time.Now().AddDate(0, 0, 14) // +14 days
	i.OccurrenceDate = time.Now()

	s, err := ctrl.model.LoadSettings(ownerID)
	if err != nil {
		return ErrInvalid(err, "Fehler beim Laden der Einstellungen")
	}
	counter, err := ctrl.model.GetMaxCounter(i.CompanyID, s.UseLocalCounter, ownerID)
	if err != nil {
		return ErrInvalid(err, "Fehler beim Ermitteln des Zählers")
	}
	i.Counter = counter + 1
	company, err := ctrl.model.LoadCompany(i.CompanyID, ownerID)
	if err != nil {
		return ErrInvalid(err, "Kann Firma nicht laden")
	}
	i.Number = formatInvoiceNumber(s.InvoiceNumberTemplate, company.Kundennummer, int(i.Counter))
	// update all invoice positions: set ID to 0
	for idx := range i.InvoicePositions {
		i.InvoicePositions[idx].ID = 0
	}

	m["title"] = "Neue Rechnung anlegen"
	m["invoice"] = i
	m["company"] = company
	m["submit"] = "Rechnung erstellen"
	m["action"] = "/invoice/new"
	m["cancel"] = fmt.Sprintf("/company/%d", i.CompanyID)

	return c.Render(http.StatusOK, "invoiceedit.html", m)
}

func (ctrl *controller) invoiceEdit(c echo.Context) error {
	m := ctrl.defaultResponseMap(c, "Rechnung bearbeiten")
	ownerID := c.Get("ownerid").(uint)
	switch c.Request().Method {
	case http.MethodGet:
		i, err := ctrl.model.LoadInvoice(c.Param("id"), ownerID)
		if err != nil {
			return ErrInvalid(err, "Kann Rechnung nicht laden")
		}
		var cpy *model.Company
		if cpy, err = ctrl.model.LoadCompany(i.CompanyID, ownerID); err != nil {
			return ErrInvalid(err, "Kann Firma nicht laden")
		}
		m["title"] = "Rechnung " + i.Number
		m["invoice"] = i
		m["company"] = cpy
		m["submit"] = "Rechnung speichern"
		m["action"] = "/invoice/edit/" + c.Param("id")
		m["cancel"] = "/invoice/detail/" + c.Param("id")
		return c.Render(http.StatusOK, "invoiceedit.html", m)
	case http.MethodPost:
		mi, err := bindInvoice(c)
		if err != nil {
			return ErrInvalid(err, "Fehler beim Verarbeiten der Eingabedaten")
		}
		if err = ctrl.model.UpdateInvoice(mi, ownerID); err != nil {
			return ErrInvalid(err, "Fehler beim Speichern der Rechnung")
		}
		return c.Redirect(http.StatusSeeOther, "/invoice/detail/"+c.Param("id"))
	}
	return nil
}

func (ctrl *controller) getXMLPathForInvoice(inv *model.Invoice) string {
	return filepath.Join(ctrl.model.Config.XMLDir, fmt.Sprintf("user%d", inv.OwnerID), fmt.Sprintf("%d.xml", inv.ID))
}

func (ctrl *controller) getPDFPathForInvoice(inv *model.Invoice) string {
	return filepath.Join(ctrl.model.Config.XMLDir, fmt.Sprintf("user%d", inv.OwnerID), fmt.Sprintf("%d.pdf", inv.ID))
}

func (ctrl *controller) invoiceZUGFeRDXML(c echo.Context) error {
	ownerID := c.Get("ownerid").(uint)
	i, err := ctrl.model.LoadInvoice(c.Param("id"), ownerID)
	if err != nil {
		return ErrInvalid(err, "Kann Rechnung nicht laden")
	}
	outPath := ctrl.getXMLPathForInvoice(i)
	if err = ensureDir(filepath.Dir(outPath)); err != nil {
		return ErrInvalid(err, "Fehler beim Erstellen des Verzeichnisses für die XML-Datei")
	}
	err = ctrl.model.CreateZUGFeRDXML(i, ownerID, outPath)
	if err != nil {
		return ErrInvalid(err, "Fehler beim Erstellen der ZUGFeRD XML")
	}

	return c.Attachment(outPath, fmt.Sprintf("%s.xml", i.Number))
}

func ensureDir(dirName string) error {
	err := os.MkdirAll(dirName, 0755)
	if err != nil {
		return err
	}
	return nil
}

func (ctrl *controller) invoiceZUGFeRDPDF(c echo.Context) error {
	logger := c.Get("logger").(*slog.Logger)
	ownerid := c.Get("ownerid").(uint)
	i, err := ctrl.model.LoadInvoice(c.Param("id"), ownerid)
	if err != nil {
		return ErrInvalid(err, "Kann Rechnung nicht laden")
	}
	xmlPath := ctrl.getXMLPathForInvoice(i)

	err = ctrl.model.CreateZUGFeRDXML(i, ownerid, xmlPath)
	if err != nil {
		return ErrInvalid(err, "Fehler beim Erstellen der ZUGFeRD XML")
	}
	pdfPath := ctrl.getPDFPathForInvoice(i)
	pdfname := fmt.Sprintf("%s.pdf", i.Number)

	// make directory user if not exists
	userdir := filepath.Join(ctrl.model.Config.XMLDir, fmt.Sprintf("user%d", ownerid))
	err = ensureDir(userdir)
	if err != nil {
		return ErrInvalid(err, "Fehler beim Erstellen des Verzeichnisses für den Benutzer")
	}

	err = ctrl.model.CreateZUGFeRDPDF(i, 1, xmlPath, pdfPath, logger)
	if err != nil {
		return ErrInvalid(err, "Fehler beim Erstellen der ZUGFeRD PDF")
	}

	return c.Attachment(pdfPath, pdfname)
}
