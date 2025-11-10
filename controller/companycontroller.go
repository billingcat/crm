package controller

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/billingcat/crm/model"
	"github.com/go-playground/form/v4"
	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
	"github.com/xuri/excelize/v2"
)

func (ctrl *controller) companyInit(e *echo.Echo) {
	g := e.Group("/company")
	g.Use(ctrl.authMiddleware)
	g.GET("/new", ctrl.companynew)
	g.GET("/list", ctrl.companylist)
	g.GET("/list/export", ctrl.companyExport)
	g.POST("/new", ctrl.companynew)
	g.GET("/edit/:id", ctrl.companyedit)
	g.POST("/edit/:id", ctrl.companyedit)
	g.GET("/:id/:name", ctrl.companydetail)
	g.GET("/:id", ctrl.companydetail)
	g.POST("/:id/tags", ctrl.companyTagsUpdate)
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
	CustomerNumber         string            `form:"customer_number"`
	EmailInvoice           string            `form:"emailinvoice"`
	SupplierNumber         string            `form:"suppliernumber"`
	ContactInvoice         string            `form:"contactinvoice"`
	DefaultTaxRate         string            `form:"defaulttaxrate"`
	Address1               string            `form:"address1"`
	Address2               string            `form:"address2"`
	Zip                    string            `form:"zip"`
	City                   string            `form:"city"`
	Phone                  []contactInfoForm `form:"phone"`
	Country                string            `form:"country"`
	VATID                  string            `form:"vatid"`
	InvoiceOpening         string            `form:"invoiceopening"`
	InvoiceCurrency        string            `form:"invoicecurrency"`
	InvoiceTaxType         string            `form:"invoicetaxtype"`
	InvoiceFooter          string            `form:"invoicefooter"`
	InvoiceExemptionReason string            `form:"invoiceexemptionreason"`
	Tags                   []string          `form:"tags"` // multiple inputs
}

func (ctrl *controller) companynew(c echo.Context) error {
	m := ctrl.defaultResponseMap(c, "Neue Firma anlegen")
	switch c.Request().Method {
	case http.MethodGet:
		m["submit"] = "Firma anlegen"
		m["action"] = "/company/new"
		m["cancel"] = "/"

		// Optional: show a formatted suggestion in the placeholder
		// (non-persistent; real allocation happens in NextCustomerNumberTx)
		suggestion, _ := ctrl.model.SuggestNextCustomerNumber(c.Request().Context())
		m["company"] = model.Company{
			CustomerNumber: suggestion, // used as placeholder in your template
		}
		return c.Render(http.StatusOK, "companyedit.html", m)

	case http.MethodPost:
		ownerID := c.Get("ownerid").(uint)

		// Decode form
		if err := c.Request().ParseForm(); err != nil {
			return ErrInvalid(err, "Fehler beim Verarbeiten der Formulardaten")
		}
		var comp companyForm
		dec := form.NewDecoder()
		if err := dec.Decode(&comp, c.Request().Form); err != nil {
			return ErrInvalid(err, "Fehler beim Verarbeiten der Formulardaten")
		}

		dbCompany := &model.Company{
			Address1:               strings.TrimSpace(comp.Address1),
			Address2:               strings.TrimSpace(comp.Address2),
			Background:             strings.TrimSpace(comp.Background),
			ContactInvoice:         strings.TrimSpace(comp.ContactInvoice),
			InvoiceCurrency:        strings.TrimSpace(comp.InvoiceCurrency),
			InvoiceExemptionReason: strings.TrimSpace(comp.InvoiceExemptionReason),
			InvoiceFooter:          strings.TrimSpace(comp.InvoiceFooter),
			InvoiceOpening:         strings.TrimSpace(comp.InvoiceOpening),
			InvoiceTaxType:         strings.TrimSpace(comp.InvoiceTaxType),
			CustomerNumber:         strings.TrimSpace(comp.CustomerNumber), // handled below
			Country:                strings.TrimSpace(comp.Country),
			Name:                   strings.TrimSpace(comp.Name),
			City:                   strings.TrimSpace(comp.City),
			OwnerID:                ownerID,
			Zip:                    strings.TrimSpace(comp.Zip),
			InvoiceEmail:           strings.TrimSpace(comp.EmailInvoice),
			SupplierNumber:         strings.TrimSpace(comp.SupplierNumber),
			VATID:                  strings.TrimSpace(comp.VATID),
		}

		// ContactInfos (trim)
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
				ParentType: model.ParentTypeCompany,
			}
			dbCompany.ContactInfos = append(dbCompany.ContactInfos, dbCI)
		}

		// VAT
		var err error
		dbCompany.DefaultTaxRate, err = decimal.NewFromString(strings.TrimSpace(comp.DefaultTaxRate))
		if err != nil {
			return ErrInvalid(err, "Fehler beim Verarbeiten der Mehrwertsteuer")
		}

		// ---- Customer number handling (model-only) ----
		// Rule:
		// - Empty -> allocate via NextCustomerNumberTx
		// - Non-empty -> must be free (or same record; here new => excludeID=0) and may lift the counter
		if dbCompany.CustomerNumber == "" {
			num, _, allocErr := ctrl.model.NextCustomerNumberTx(c.Request().Context())
			if allocErr != nil {
				return ErrInvalid(allocErr, "Kundennummer konnte nicht automatisch vergeben werden")
			}
			dbCompany.CustomerNumber = num
		} else {
			ok, msg, chkErr := ctrl.model.CheckCustomerNumber(c.Request().Context(), dbCompany.CustomerNumber, 0 /* excludeID for new = 0 */)
			if chkErr != nil {
				return ErrInvalid(chkErr, "Fehler bei der Kundennummernprüfung")
			}
			if !ok {
				if msg == "" {
					msg = "Kundennummer bereits vergeben"
				}
				return ErrInvalid(fmt.Errorf("customer number taken"), msg)
			}
			// Try to lift the counter when the provided value is ahead of settings
			if liftErr := ctrl.model.MaybeLiftCustomerCounterFor(c.Request().Context(), dbCompany.CustomerNumber); liftErr != nil {
				// non-fatal vs. fatal is your choice; I make it fatal to keep invariants strong
				return ErrInvalid(liftErr, "Konnte Zählerstand nicht anheben")
			}
		}
		// ----------------------------------------------

		tagNames := normalizeSliceInput(comp.Tags)

		if err := ctrl.model.SaveCompany(dbCompany, ownerID, tagNames); err != nil {
			return ErrInvalid(err, "Fehler beim Speichern der Firma")
		}
		return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/company/%d", dbCompany.ID))
	}
	return fmt.Errorf("Unknown method %s", c.Request().Method)
}

func (ctrl *controller) companydetail(c echo.Context) error {
	m := ctrl.defaultResponseMap(c, "Kontakt Details")
	ownerID := c.Get("ownerid").(uint)

	// Load company
	companyDB, err := ctrl.model.LoadCompany(c.Param("id"), ownerID)
	if err != nil {
		return ErrInvalid(err, "Kann Firma nicht laden")
	}

	// Load notes
	notes, err := ctrl.model.LoadAllNotesForParent(ownerID, model.ParentTypeCompany, companyDB.ID)
	if err != nil {
		return ErrInvalid(err, "Kann Notizen nicht laden")
	}

	// Load tags for inline editing
	tags, err := ctrl.model.ListTagsForParent(ownerID, model.ParentTypeCompany, companyDB.ID)
	if err != nil {
		return ErrInvalid(err, "Kann Tags nicht laden")
	}
	tagNames := make([]string, 0, len(tags))
	for _, t := range tags {
		tagNames = append(tagNames, t.Name)
	}

	// Template data
	m["notes"] = notes
	m["right"] = "companydetail"
	m["companydetail"] = companyDB
	m["title"] = companyDB.Name
	m["ExistingTags"] = tagNames
	m["noteparenttype"] = model.ParentTypeCompany

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
		// Decode form
		if err := c.Request().ParseForm(); err != nil {
			return ErrInvalid(err, "Fehler beim Verarbeiten der Formulardaten")
		}
		var comp companyForm
		dec := form.NewDecoder()
		if err := dec.Decode(&comp, c.Request().Form); err != nil {
			return ErrInvalid(err, "Fehler beim Verarbeiten der Formulardaten")
		}

		// Load DB object
		paramCompanyID := c.Param("id")
		dbCompany, err := ctrl.model.LoadCompany(paramCompanyID, ownerID)
		if err != nil {
			return ErrInvalid(fmt.Errorf("cannot find company with id %v and ownerid %v", paramCompanyID, ownerID), "Kann Firma nicht laden")
		}

		// Update basic fields (trim inputs where appropriate)
		dbCompany.Background = strings.TrimSpace(comp.Background)
		dbCompany.Name = strings.TrimSpace(comp.Name)
		// Customer number handled below
		dbCompany.Address1 = strings.TrimSpace(comp.Address1)
		dbCompany.Address2 = strings.TrimSpace(comp.Address2)
		dbCompany.InvoiceEmail = strings.TrimSpace(comp.EmailInvoice)
		dbCompany.SupplierNumber = strings.TrimSpace(comp.SupplierNumber)
		dbCompany.ContactInvoice = strings.TrimSpace(comp.ContactInvoice)
		dbCompany.City = strings.TrimSpace(comp.City)
		dbCompany.Zip = strings.TrimSpace(comp.Zip)
		dbCompany.VATID = strings.TrimSpace(comp.VATID)
		dbCompany.Country = strings.TrimSpace(comp.Country)
		dbCompany.InvoiceOpening = strings.TrimSpace(comp.InvoiceOpening)
		dbCompany.InvoiceCurrency = strings.TrimSpace(comp.InvoiceCurrency)
		dbCompany.InvoiceTaxType = strings.TrimSpace(comp.InvoiceTaxType)
		dbCompany.InvoiceFooter = strings.TrimSpace(comp.InvoiceFooter)
		dbCompany.InvoiceExemptionReason = strings.TrimSpace(comp.InvoiceExemptionReason)

		if dbCompany.DefaultTaxRate, err = decimal.NewFromString(strings.TrimSpace(comp.DefaultTaxRate)); err != nil {
			return ErrInvalid(err, "Fehler beim Verarbeiten der Mehrwertsteuer")
		}

		// --- Customer number handling (model-only) ---
		// Rule for edit:
		// - Empty input => keep current db value (no change)
		// - Non-empty and different => must be available (excluding this company),
		//   and lift counter if the numeric part is ahead of settings.
		desiredNumber := strings.TrimSpace(comp.CustomerNumber)
		if desiredNumber != "" && desiredNumber != dbCompany.CustomerNumber {
			ok, msg, chkErr := ctrl.model.CheckCustomerNumber(c.Request().Context(), desiredNumber, dbCompany.ID)
			if chkErr != nil {
				return ErrInvalid(chkErr, "Fehler bei der Kundennummernprüfung")
			}
			if !ok {
				if msg == "" {
					msg = "Kundennummer bereits vergeben"
				}
				return ErrInvalid(fmt.Errorf("customer number taken"), msg)
			}
			if liftErr := ctrl.model.MaybeLiftCustomerCounterFor(c.Request().Context(), desiredNumber); liftErr != nil {
				return ErrInvalid(liftErr, "Konnte Zählerstand nicht anheben")
			}
			dbCompany.CustomerNumber = desiredNumber
		}
		// ------------------------------------------------

		// Replace contact infos (simple strategy: rebuild list)
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
				ParentType: model.ParentTypeCompany,
			}
			dbCompany.ContactInfos = append(dbCompany.ContactInfos, dbCI)
		}

		if err := ctrl.model.SaveCompany(dbCompany, ownerID, comp.Tags); err != nil {
			return ErrInvalid(err, "Fehler beim Speichern der Firma")
		}
		if err = c.Redirect(http.StatusSeeOther, fmt.Sprintf("/company/%d/%s", dbCompany.ID, dbCompany.Name)); err != nil {
			return ErrInvalid(err, "Fehler beim Weiterleiten zur Firmenseite")
		}
	}
	return nil
}
func normalizeSliceInput(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range in {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		norm := strings.ToLower(t)
		if !seen[norm] {
			seen[norm] = true
			out = append(out, t)
		}
	}
	return out
}

// POST /company/:id/tags
func (ctrl *controller) companyTagsUpdate(c echo.Context) error {
	ownerID := c.Get("ownerid").(uint)
	companyID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return ErrInvalid(err, "invalid company ID")
	}

	// Parse tags from form (multiple name="tags" inputs)
	if err := c.Request().ParseForm(); err != nil {
		return ErrInvalid(err, "failed to parse form")
	}
	tagNames := normalizeSliceInput(c.Request().Form["tags"])

	// Replace tags transactionally
	if err := ctrl.model.ReplaceCompanyTagsByName(uint(companyID), ownerID, tagNames); err != nil {
		return ErrInvalid(err, "error updating tags")
	}

	// If this is an AJAX call, return JSON so the page can update without reload
	if c.Request().Header.Get("HX-Request") != "" || c.Request().Header.Get("X-Requested-With") == "XMLHttpRequest" {
		return c.JSON(http.StatusOK, echo.Map{"status": "ok"})
	}

	// Fallback redirect for normal form submit
	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/company/%d", companyID))
}

func (ctrl *controller) companylist(c echo.Context) error {
	ownerID := c.Get("ownerid").(uint)

	// Inputs
	q := strings.TrimSpace(c.QueryParam("q"))
	tags := c.QueryParams()["tags"] // multiple tags
	mode := strings.ToLower(strings.TrimSpace(c.QueryParam("mode")))
	modeAND := (mode == "and")

	// Pagination
	const defaultPageSize = 25
	page, _ := strconv.Atoi(c.QueryParam("p"))
	if page <= 0 {
		page = 1
	}
	ps, _ := strconv.Atoi(c.QueryParam("ps"))
	if ps <= 0 {
		ps = defaultPageSize
	}
	offset := (page - 1) * ps

	// Model calls (no DB in controller)
	allTags, err := ctrl.model.ListOwnerCompanyTags(ownerID)
	if err != nil {
		return ErrInvalid(err, "Fehler beim Laden der Tags")
	}

	res, err := ctrl.model.SearchCompaniesByTags(ownerID, model.CompanyListFilters{
		Query:   q,
		Tags:    normalizeSliceInput(tags),
		ModeAND: modeAND,
		Limit:   ps,
		Offset:  offset,
	})
	if err != nil {
		return ErrInvalid(err, "Fehler beim Laden der Firmenliste")
	}

	// View model
	m := ctrl.defaultResponseMap(c, "Kunden")
	m["title"] = "Kunden"
	m["right"] = "customers"
	m["q"] = q
	m["selectedTags"] = normalizeSliceInput(tags)
	m["modeAND"] = modeAND
	m["tagCounts"] = allTags
	m["companies"] = res.Companies
	m["page"] = int64(page)
	m["pagesize"] = int64(ps)
	m["total"] = res.Total

	return c.Render(http.StatusOK, "customerlist.html", m)
}

// tagsForParent returns all active tags for a given entity (parent type + ID).
// Usage in templates: {{ range (tagsForParent $.OwnerID "company" .ID) }} ... {{ end }}
func (ctrl *controller) tagsForParent(ownerID any, parentType string, parentID any) []model.Tag {
	oid, ok1 := ownerID.(uint)
	if !ok1 {
		switch v := ownerID.(type) {
		case int:
			oid = uint(v)
		case int64:
			oid = uint(v)
		case string:
			if n, err := strconv.Atoi(v); err == nil {
				oid = uint(n)
			}
		default:
			return nil
		}
	}

	var pid uint
	switch v := parentID.(type) {
	case uint:
		pid = v
	case int:
		pid = uint(v)
	case int64:
		pid = uint(v)
	case string:
		if n, err := strconv.Atoi(v); err == nil {
			pid = uint(n)
		}
	default:
		return nil
	}

	tags, err := ctrl.model.ListTagsForParent(oid, parentType, pid)
	if err != nil {
		return []model.Tag{}
	}
	return tags
}

func (ctrl *controller) companyExport(c echo.Context) error {
	ownerID := c.Get("ownerid").(uint)

	format := strings.ToLower(strings.TrimSpace(c.QueryParam("format"))) // "csv" or "excel"
	if format == "" {
		format = "csv"
	}

	q := strings.TrimSpace(c.QueryParam("q"))
	tags := normalizeSliceInput(c.QueryParams()["tags"])
	modeAND := strings.ToLower(c.QueryParam("mode")) == "and"

	// Fetch ALL filtered companies (ignores pagination)
	res, err := ctrl.model.ListAllCompaniesByTags(ownerID, model.CompanyListFilters{
		Query:   q,
		Tags:    tags,
		ModeAND: modeAND,
	})
	if err != nil {
		return ErrInvalid(err, "Fehler beim Laden der Firmen für den Export")
	}

	// Load tags per company (for a friendly "Tags" column)
	ids := make([]uint, 0, len(res))
	for _, cmp := range res {
		ids = append(ids, cmp.ID)
	}
	tagMap, _ := ctrl.model.TagsForCompanies(ownerID, ids)

	// Filename with timestamp
	stamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("firmen-%s", stamp)
	if q != "" || len(tags) > 0 {
		filename = fmt.Sprintf("firmen-filter-%s", stamp)
	}

	switch format {
	case "excel", "xlsx", "xls":
		return exportCompaniesExcel(c, filename+".xlsx", res, tagMap)
	default:
		return exportCompaniesCSV(c, filename+".csv", res, tagMap)
	}
}

func exportCompaniesCSV(c echo.Context, filename string, rows []model.Company, tagMap map[uint][]model.Tag) error {
	c.Response().Header().Set(echo.HeaderContentType, "text/csv; charset=utf-8")
	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf("attachment; filename=%q", filename))

	w := csv.NewWriter(c.Response())
	defer w.Flush()

	// Header
	_ = w.Write([]string{"ID", "Name", "City", "Country", "Tags"})

	for _, cmp := range rows {
		// Build tag string "A; B; C"
		var names []string
		if ts, ok := tagMap[cmp.ID]; ok {
			for _, t := range ts {
				names = append(names, t.Name)
			}
		}
		sort.Strings(names)
		tagStr := strings.Join(names, "; ")

		_ = w.Write([]string{
			fmt.Sprintf("%d", cmp.ID),
			strings.TrimSpace(cmp.Name),
			strings.TrimSpace(cmp.Zip + " " + cmp.City),
			strings.TrimSpace(cmp.Country),
			tagStr,
		})
	}
	return nil
}

func exportCompaniesExcel(c echo.Context, filename string, rows []model.Company, tagMap map[uint][]model.Tag) error {
	f := excelize.NewFile()
	defer f.Close()

	sheet := f.GetSheetName(0)

	// Header
	header := []string{"ID", "Name", "City", "Country", "Tags"}
	for i, h := range header {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
	}
	// Bold header
	styleID, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
	})
	_ = f.SetCellStyle(sheet, "A1", "E1", styleID)

	// Rows
	for r, cmp := range rows {
		row := r + 2
		var names []string
		if ts, ok := tagMap[cmp.ID]; ok {
			for _, t := range ts {
				names = append(names, t.Name)
			}
		}
		sort.Strings(names)
		tagStr := strings.Join(names, "; ")
		_ = f.SetCellValue(sheet, cell(row, 1), cmp.ID)
		_ = f.SetCellValue(sheet, cell(row, 2), cmp.Name)
		_ = f.SetCellValue(sheet, cell(row, 3), fmt.Sprintf("%s %s", cmp.Zip, cmp.City))
		_ = f.SetCellValue(sheet, cell(row, 4), cmp.Country)
		_ = f.SetCellValue(sheet, cell(row, 5), tagStr)
	}
	// Basic niceties
	lastRow := len(rows) + 1
	_ = f.AutoFilter(sheet, fmt.Sprintf("A1:E%d", lastRow), nil)
	_ = f.SetPanes(sheet, &excelize.Panes{
		Freeze:      true,
		YSplit:      1, // eine Zeile einfrieren
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	})
	_ = f.SetColWidth(sheet, "A", "E", 18)

	// Serve
	c.Response().Header().Set(echo.HeaderContentType,
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf("attachment; filename=%q", filename))
	return f.Write(c.Response())
}

// helpers
func cell(row, col int) string {
	addr, _ := excelize.CoordinatesToCellName(col, row)
	return addr
}
