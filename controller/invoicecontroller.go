package controller

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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
	g.POST("/status/:id", ctrl.invoiceStatusChange)

	lg := e.Group("/invoices", ctrl.authMiddleware)
	lg.GET("", ctrl.invoiceList)
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
	if inv.Status != model.InvoiceStatusDraft {
		return echo.NewHTTPError(http.StatusForbidden, "invoice cannot be deleted after issuing")
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
	i, err := ctrl.model.LoadInvoice(c.Param("id"), ownerID)
	if err != nil {
		return ErrInvalid(err, "Kann Rechnung nicht laden")
	}
	if i.Status != model.InvoiceStatusDraft {
		return echo.NewHTTPError(http.StatusForbidden, "invoice is not editable after issuing")
	}
	switch c.Request().Method {
	case http.MethodGet:
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

// getXMLPathForInvoice returns the full path where the XML for the invoice is stored
func (ctrl *controller) getXMLPathForInvoice(inv *model.Invoice) string {
	return filepath.Join(ctrl.model.Config.XMLDir, fmt.Sprintf("user%d", inv.OwnerID), fmt.Sprintf("%d.xml", inv.ID))
}

// getPDFPathForInvoice returns the full path where the PDF for the invoice is stored
func (ctrl *controller) getPDFPathForInvoice(inv *model.Invoice) string {
	return filepath.Join(ctrl.model.Config.XMLDir, fmt.Sprintf("user%d", inv.OwnerID), fmt.Sprintf("%d.pdf", inv.ID))
}

func (ctrl *controller) invoiceZUGFeRDXML(c echo.Context) error {
	ownerID := c.Get("ownerid").(uint)
	logger := c.Get("logger").(*slog.Logger)

	i, err := ctrl.model.LoadInvoice(c.Param("id"), ownerID)
	if err != nil {
		return ErrInvalid(err, "Kann Rechnung nicht laden")
	}
	outPath := ctrl.getXMLPathForInvoice(i)
	userFilename := fmt.Sprintf("%s.xml", i.Number)
	// when not draft, just send existing file if exists
	if i.Status != model.InvoiceStatusDraft {
		if _, err = os.Stat(outPath); err == nil {
			logger.Info("re-using existing zugferd xml", "invoice_id", i.ID, "path", outPath)
			return c.Attachment(outPath, userFilename)
		}
		logger.Info("zugferd xml not found, re-creating", "invoice_id", i.ID, "path", outPath)
	}
	if err = ensureDir(filepath.Dir(outPath)); err != nil {
		return ErrInvalid(err, "Fehler beim Erstellen des Verzeichnisses für die XML-Datei")
	}
	err = ctrl.model.CreateZUGFeRDXML(i, ownerID, outPath)
	if err != nil {
		return ErrInvalid(err, "Fehler beim Erstellen der ZUGFeRD XML")
	}

	return c.Attachment(outPath, userFilename)
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

	pdfname := fmt.Sprintf("%s.pdf", i.Number)

	// when not draft, just send existing file if exists
	if i.Status != model.InvoiceStatusDraft {
		pdfPath := ctrl.getPDFPathForInvoice(i)
		if _, err = os.Stat(pdfPath); err == nil {
			logger.Info("re-using existing zugferd pdf", "invoice_id", i.ID, "path", pdfPath)
			return c.Attachment(pdfPath, pdfname)
		}
		logger.Info("zugferd pdf not found, re-creating", "invoice_id", i.ID, "path", pdfPath)
	}

	xmlPath := ctrl.getXMLPathForInvoice(i)

	err = ctrl.model.CreateZUGFeRDXML(i, ownerid, xmlPath)
	if err != nil {
		return ErrInvalid(err, "Fehler beim Erstellen der ZUGFeRD XML")
	}
	pdfPath := ctrl.getPDFPathForInvoice(i)

	// make directory user if not exists
	userdir := filepath.Join(ctrl.model.Config.XMLDir, fmt.Sprintf("user%d", ownerid))
	err = ensureDir(userdir)
	if err != nil {
		return ErrInvalid(err, "Fehler beim Erstellen des Verzeichnisses für den Benutzer")
	}

	err = ctrl.model.CreateZUGFeRDPDF(i, ownerid, xmlPath, pdfPath, logger)
	if err != nil {
		return ErrInvalid(err, "Fehler beim Erstellen der ZUGFeRD PDF")
	}

	return c.Attachment(pdfPath, pdfname)
}

func (ctrl *controller) invoiceStatusChange(c echo.Context) error {
	ownerID := c.Get("ownerid").(uint)

	// parse invoice id
	idStr := c.Param("id")
	id64, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || id64 == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid invoice id")
	}
	invoiceID := uint(id64)

	// read desired status
	desired := strings.TrimSpace(c.FormValue("status"))
	if desired == "" {
		// fallback: allow JSON too, though dein Frontend sendet x-www-form-urlencoded
		var payload struct {
			Status string `json:"status"`
		}
		if bindErr := c.Bind(&payload); bindErr == nil && payload.Status != "" {
			desired = payload.Status
		}
	}
	dest, ok := toInvoiceStatus(desired)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid status value")
	}

	now := time.Now()

	// execute transition
	switch dest {
	case model.InvoiceStatusIssued:
		err = ctrl.model.MarkInvoiceIssued(invoiceID, ownerID, now)
	case model.InvoiceStatusPaid:
		err = ctrl.model.MarkInvoicePaid(invoiceID, ownerID, now)
	case model.InvoiceStatusVoided:
		err = ctrl.model.VoidInvoice(invoiceID, ownerID, now)
	case model.InvoiceStatusDraft:
		err = ctrl.model.MarkInvoiceDraft(invoiceID, ownerID, now)
	default:
		return echo.NewHTTPError(http.StatusBadRequest, "unsupported transition")
	}
	if err != nil {
		// Gib dem Nutzer eine klare Meldung (z.B. „paid invoices cannot be voided“)
		slog.Error("invoice status change failed", "invoice_id", invoiceID, "err", err)
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	// AJAX: kein Reload nötig – 204 reicht (Frontend checkt nur res.ok)
	// Wenn du später Zeiten zurückgeben willst, könntest du 200 + JSON senden.
	// reload auslassen, aber Daten mitschicken
	inv, loadErr := ctrl.model.LoadInvoice(invoiceID, ownerID)
	if loadErr != nil {
		return c.NoContent(http.StatusNoContent) // still ok – UI bleibt konsistent
	}

	// render PDF and XML in background, ignore errors
	go func() {
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		xmlPath := ctrl.getXMLPathForInvoice(inv)
		if err = ctrl.model.CreateZUGFeRDXML(inv, ownerID, xmlPath); err != nil {
			logger.Error("creating zugferd xml failed", "invoice_id", invoiceID, "err", err)
			return
		}
		pdfPath := ctrl.getPDFPathForInvoice(inv)
		if err = ctrl.model.CreateZUGFeRDPDF(inv, ownerID, xmlPath, pdfPath, logger); err != nil {
			logger.Error("creating zugferd pdf failed", "invoice_id", invoiceID, "err", err)
			return
		}
	}()

	type resp struct {
		Status   string  `json:"status"`
		IssuedAt *string `json:"issued_at"`
		PaidAt   *string `json:"paid_at"`
		VoidedAt *string `json:"voided_at"`
	}
	fmtTS := func(t *time.Time) *string {
		if t == nil {
			return nil
		}
		s := t.Format("02.01.2006")
		return &s
	}
	return c.JSON(http.StatusOK, resp{
		Status:   string(inv.Status),
		IssuedAt: fmtTS(inv.IssuedAt),
		PaidAt:   fmtTS(inv.PaidAt),
		VoidedAt: fmtTS(inv.VoidedAt),
	})
}

// helper: sanitize / map string -> model.InvoiceStatus
func toInvoiceStatus(s string) (model.InvoiceStatus, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "draft":
		return model.InvoiceStatusDraft, true
	case "issued":
		return model.InvoiceStatusIssued, true
	case "paid":
		return model.InvoiceStatusPaid, true
	case "voided":
		return model.InvoiceStatusVoided, true
	default:
		return "", false
	}
}

func (ctrl *controller) invoiceList(c echo.Context) error {
	ownerID := c.Get("ownerid").(uint)
	title := "Rechnungen"
	status := strings.ToLower(c.QueryParam("status"))
	format := strings.ToLower(c.QueryParam("format"))

	// --- Status-Mapping ---
	var statuses []model.InvoiceStatus
	switch status {
	case "open":
		title = "Offene Rechnungen"
		statuses = []model.InvoiceStatus{model.InvoiceStatusIssued}
	case "draft":
		title = "Entwürfe"
		statuses = []model.InvoiceStatus{model.InvoiceStatusDraft}
	case "issued":
		title = "Ausgestellte Rechnungen"
		statuses = []model.InvoiceStatus{model.InvoiceStatusIssued}
	case "paid":
		title = "Bezahlte Rechnungen"
		statuses = []model.InvoiceStatus{model.InvoiceStatusPaid}
	case "voided":
		title = "Stornierte Rechnungen"
		statuses = []model.InvoiceStatus{model.InvoiceStatusVoided}
	default:
		title = "Alle Rechnungen"
		// any → kein Status-Filter
	}

	// --- CompanyID optional ---
	var companyID *uint
	if cid := c.QueryParam("company_id"); cid != "" {
		if v, err := strconv.ParseUint(cid, 10, 64); err == nil {
			tmp := uint(v)
			companyID = &tmp
		}
	}

	// --- Zeitraum ---
	periodField := strings.ToLower(c.QueryParam("period_field"))
	if periodField != "due" {
		periodField = "date"
	}
	parseDate := func(s string) *time.Time {
		if s == "" {
			return nil
		}
		if t, err := time.Parse("2006-01-02", s); err == nil {
			return &t
		}
		if t, err := time.Parse("02.01.2006", s); err == nil {
			return &t
		}
		return nil
	}
	dateFrom := parseDate(c.QueryParam("date_from"))
	dateTo := parseDate(c.QueryParam("date_to"))

	// --- Sortierung ---
	order := "date desc, id desc"
	switch strings.ToLower(c.QueryParam("sort")) {
	case "date_asc":
		order = "date asc, id asc"
	case "due_asc":
		order = "due_date asc, id asc"
	case "due_desc":
		order = "due_date desc, id desc"
	case "total_asc":
		order = "gross_total asc, id asc"
	case "total_desc":
		order = "gross_total desc, id desc"
	}

	// --- Pagination ---
	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.QueryParam("page_size"))
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}
	offset := (page - 1) * pageSize

	// --- Daten via Repo laden ---
	rows, total, err := ctrl.model.FindInvoices(
		ownerID,
		statuses,
		companyID,
		periodField,
		dateFrom,
		dateTo,
		pageSize,
		offset,
		order,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "query_failed"})
	}

	// --- JSON-Ausgabe ---
	if format == "json" || strings.Contains(c.Request().Header.Get("Accept"), "application/json") {
		type item struct {
			ID         uint                `json:"id"`
			CompanyID  uint                `json:"company_id"`
			Number     string              `json:"number"`
			Date       string              `json:"date"`
			DueDate    string              `json:"due_date"`
			Status     model.InvoiceStatus `json:"status"`
			GrossTotal int64               `json:"gross_total"`
		}
		out := make([]item, 0, len(rows))
		for _, r := range rows {
			out = append(out, item{
				ID:         r.ID,
				CompanyID:  r.CompanyID,
				Number:     r.Number,
				Date:       r.Date.Format("02.01.2006"),
				DueDate:    r.DueDate.Format("02.01.2006"),
				Status:     r.Status,
				GrossTotal: r.GrossTotal.IntPart(),
			})
		}
		return c.JSON(http.StatusOK, map[string]any{
			"total": total, "page": page, "page_size": pageSize, "items": out,
		})
	}

	// --- HTML-Render ---
	m := ctrl.defaultResponseMap(c, title)
	m["invoices"] = rows
	m["total"] = total
	m["page"] = page
	m["page_size"] = pageSize
	m["isViewActive"] = (status == "open") // für aktiven Menüpunkt
	return c.Render(http.StatusOK, "invoicelist.html", m)
}
