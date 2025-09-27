package controller

import (
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/billingcat/crm/model"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/shopspring/decimal"
	"github.com/xeonx/timeago"
)

type Flash struct {
	Kind    string // "success" | "error" | "warning" | "info"
	Message string
}

// FlashLoader zieht Flashes aus der Session (und leert sie) und legt sie in echo.Context.
func FlashLoader(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		sess, _ := session.Get("session", c)
		raw := sess.Flashes() // liest & leert
		_ = sess.Save(c.Request(), c.Response())

		flashes := make([]Flash, 0, len(raw))
		for _, it := range raw {
			if f, ok := it.(Flash); ok {
				flashes = append(flashes, f)
			}
		}
		c.Set("flashes", flashes)
		return next(c)
	}
}

// AddFlash setzt eine Flash-Message (nutzt Gorilla Sessions via echo-contrib/session).
func AddFlash(c echo.Context, kind, msg string) error {
	sess, _ := session.Get("session", c)
	sess.AddFlash(Flash{Kind: kind, Message: msg})
	if err := sess.Save(c.Request(), c.Response()); err != nil {
		return ErrInvalid(err, "Fehler beim Speichern der Session")
	}
	return nil
}

type appError struct {
	Code   string // stabiler, interner Fehlercode für Ops/Support
	Status int    // passender HTTP-Status
	Err    error  // ursprünglicher Fehler (wird nie an den Client gegeben)
	Public string // sicherer Text für Nutzer (optional)
}

func (e *appError) Error() string { return fmt.Sprintf("%s: %v", e.Code, e.Err) }
func (e *appError) Unwrap() error { return e.Err }

// Hilfsfunktionen zum Bauen typischer Fehler
func ErrNotFound(err error) *appError {
	return &appError{Code: "NOT_FOUND", Status: http.StatusNotFound, Err: err}
}
func ErrInvalid(err error, public string) *appError {
	return &appError{Code: "INVALID_INPUT", Status: http.StatusBadRequest, Err: err, Public: public}
}
func ErrInternal(err error) *appError {
	return &appError{Code: "INTERNAL", Status: http.StatusInternalServerError, Err: err}
}

var (
	timeagoGerman = timeago.NoMax(timeago.German)
)

// The Template interface implements rendering functionality for echo.
type Template struct {
	templates *template.Template
}

// Render is the echo way of rendering templates.
func (t *Template) Render(w io.Writer, name string, data interface{}, _ echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

type controller struct {
	model *model.CRMDatenbank
}

func (ctrl *controller) defaultResponseMap(c echo.Context, title string) map[string]any {
	responseMap := map[string]any{
		"title":    title,
		"loggedin": false,
		"path":     c.Request().URL.Path,
	}

	if flashes, ok := c.Get("flashes").([]Flash); ok {
		responseMap["flashes"] = flashes
	} else {
		responseMap["flashes"] = []Flash{}
	}

	if t := c.Get(middleware.DefaultCSRFConfig.ContextKey); t != nil {
		responseMap["CSRFToken"] = t.(string)
	}

	ownerID := c.Get("ownerid")
	userID := c.Get("uid")
	if ownerID == nil || userID == nil {
		return responseMap
	}
	responseMap["ownerid"] = ownerID
	responseMap["uid"] = userID.(uint)
	user, err := ctrl.model.GetUserByID(ownerID)
	if err != nil {
		c.Get("logger").(*slog.Logger).Warn("cannot get user by ID", "error", err)
		responseMap["uid"] = nil
		responseMap["ownerid"] = nil
		c.Set("uid", nil)
		c.Set("ownerid", nil)
		return responseMap
	}
	if user != nil {
		responseMap["email"] = user.Email
		responseMap["fullname"] = user.FullName
		responseMap["loggedin"] = true
	}

	items, err := ctrl.model.GetRecentItems(ownerID.(uint), 5)
	if err != nil {
		c.Get("logger").(*slog.Logger).Warn("cannot get recent items", "error", err)
	} else {
		responseMap["recentitems"] = items
	}
	return responseMap
}

type lastChanges struct {
	Who  string
	What template.HTML
	When time.Time
}

// kleine Helfer
func escape(s string) string { return html.EscapeString(s) }
func safeLink(href, text string) template.HTML {
	return template.HTML(fmt.Sprintf(`<a href="%s">%s</a>`, escape(href), escape(text)))
}
func snippet(s string, max int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	s = strings.Join(strings.Fields(s), " ")
	rs := []rune(s)
	if len(rs) <= max {
		return s
	}
	return string(rs[:max-1]) + "…"
}
func (ctrl *controller) root(c echo.Context) error {
	m := ctrl.defaultResponseMap(c, "Startseite")

	ownerID := c.Get("ownerid")
	userID := c.Get("uid")
	if ownerID == nil || userID == nil {
		return c.Render(http.StatusOK, "login.html", m)
	}
	owner, err := ctrl.model.GetUserByID(userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Errorf("cannot get user by ID: %w", err))
	}

	hydr, err := ctrl.model.LoadActivity(userID, 10)
	if err != nil {
		return ErrInvalid(err, "Fehler beim Laden der Änderungen")
	}

	changelog := make([]lastChanges, 0, len(hydr.Heads))

	for _, h := range hydr.Heads {
		switch h.ItemType {
		case "company":
			if c0, ok := hydr.Companies[h.ItemID]; ok {
				coLink := safeLink(
					fmt.Sprintf("/company/%d/%s", c0.ID, url.PathEscape(c0.Name)),
					c0.Name,
				)
				changelog = append(changelog, lastChanges{
					Who:  owner.FullName,
					What: template.HTML(fmt.Sprintf(`hat die Firma %s angelegt`, coLink)),
					When: c0.CreatedAt,
				})
			}
		case "invoice":
			if iv, ok := hydr.Invoices[h.ItemID]; ok {
				c0 := hydr.Companies[iv.CompanyID]
				invLink := safeLink(fmt.Sprintf("/invoice/detail/%d", iv.ID), iv.Number)
				var coLink template.HTML
				if c0.ID != 0 {
					coLink = safeLink(fmt.Sprintf("/company/%d/%s", c0.ID, url.PathEscape(c0.Name)), c0.Name)
				} else {
					coLink = template.HTML(escape(fmt.Sprintf("Firma #%d", iv.CompanyID)))
				}
				changelog = append(changelog, lastChanges{
					Who:  owner.FullName,
					What: template.HTML(fmt.Sprintf(`hat die Rechnung %s (Firma %s) erstellt`, invLink, coLink)),
					When: iv.CreatedAt,
				})
			}
		case "note":
			n, ok := hydr.Notes[h.ItemID]
			if !ok {
				continue // Note wurde evtl. gelöscht
			}

			// kurzer, sicherer Textauszug
			body := snippet(n.Body, 140)
			bodyEsc := escape(body)

			switch n.ParentType {
			case "companies":
				if c0, ok := hydr.Companies[n.ParentID]; ok {
					target := safeLink(
						fmt.Sprintf("/company/%d/%s", c0.ID, url.PathEscape(c0.Name)),
						c0.Name,
					)
					changelog = append(changelog, lastChanges{
						Who: owner.FullName,
						What: template.HTML(fmt.Sprintf(
							`hat eine Notiz zu %s erstellt: <span class="text-slate-600 italic">%s</span>`,
							target, bodyEsc,
						)),
						When: n.CreatedAt, // bleib konsistent zu ORDER BY created_at
					})
				} else {
					// Fallback, falls die Firma nicht (mehr) geladen werden konnte
					changelog = append(changelog, lastChanges{
						Who: owner.FullName,
						What: template.HTML(fmt.Sprintf(
							`hat eine Notiz zu Firma #%d erstellt: <span class="text-slate-600 italic">%s</span>`,
							n.ParentID, bodyEsc,
						)),
						When: n.CreatedAt,
					})
				}

			case "people":
				if p, ok := hydr.People[n.ParentID]; ok {
					target := safeLink(
						fmt.Sprintf("/person/%d/%s", p.ID, url.PathEscape(p.Name)),
						p.Name,
					)
					changelog = append(changelog, lastChanges{
						Who: owner.FullName,
						What: template.HTML(fmt.Sprintf(
							`hat eine Notiz zu %s erstellt: <span class="text-slate-600 italic">%s</span>`,
							target, bodyEsc,
						)),
						When: n.CreatedAt,
					})
				} else {
					changelog = append(changelog, lastChanges{
						Who: owner.FullName,
						What: template.HTML(fmt.Sprintf(
							`hat eine Notiz zu Person #%d erstellt: <span class="text-slate-600 italic">%s</span>`,
							n.ParentID, bodyEsc,
						)),
						When: n.CreatedAt,
					})
				}

			default:
				// Unbekannter Parent-Typ (sollte nicht vorkommen)
				changelog = append(changelog, lastChanges{
					Who: owner.FullName,
					What: template.HTML(fmt.Sprintf(
						`hat eine Notiz erstellt: <span class="text-slate-600 italic">%s</span>`,
						bodyEsc,
					)),
					When: n.CreatedAt,
				})
			}

		}
	}

	// heads sind bereits sortiert; falls du robustness willst:
	sort.SliceStable(changelog, func(i, j int) bool { return changelog[i].When.After(changelog[j].When) })
	if len(hydr.Companies) == 0 {
		m["nocompanies"] = true
	}
	m["lastchanges"] = changelog
	return c.Render(http.StatusOK, "main.html", m)
}

func (ctrl *controller) search(c echo.Context) error {
	var err error
	ownerID := c.Get("ownerid").(uint)
	str := strings.TrimSpace(c.QueryParam("query"))
	if str == "" {
		return c.JSON(http.StatusOK, []any{})
	}
	if str[0] == '{' {
		var data map[string]any
		if err = json.Unmarshal([]byte(str), &data); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Errorf("cannot unmarshal search query: %w", err))
		}
		if q, ok := data["query"].(string); ok {
			str = q
		} else {
			return echo.NewHTTPError(http.StatusBadRequest, "Search query must contain a 'query' field")
		}
	} else {
		// if the query is not a json string, we assume it is a simple string
		str = strings.TrimSpace(str)
	}
	if str == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Search query cannot be empty")
	}

	companies, err := ctrl.model.FindAllCompaniesWithText(str, ownerID)
	if err != nil {
		return ErrInvalid(err, "Fehler beim Suchen der Firmen")
	}
	people, err := ctrl.model.FindAllPeopleWithText(str, ownerID)
	if err != nil {
		return ErrInvalid(err, "Fehler beim Suchen der Kontakte")
	}

	type searchResult struct {
		Text   string `json:"text"`
		Action string `json:"action"`
	}

	searchResults := make([]searchResult, 0, len(companies)+len(people))

	for _, company := range companies {
		searchResults = append(searchResults, searchResult{
			Text:   fmt.Sprintf("%s", company.Name),
			Action: fmt.Sprintf("/company/%d/%s", company.ID, url.PathEscape(company.Name)),
		})
	}

	for _, person := range people {
		searchResults = append(searchResults, searchResult{
			Text:   fmt.Sprintf("%s", person.Name),
			Action: fmt.Sprintf("/person/%d/%s", person.ID, url.PathEscape(person.Name)),
		})

	}
	return c.JSON(http.StatusOK, searchResults)
}

// NewController ist der Einstiegspunkt.
func NewController(crmdb *model.CRMDatenbank) error {
	// Environment-gesteuerte Log-Details
	// Prod: JSON, Info+; Dev: Text, Debug
	var logger *slog.Logger
	if crmdb.Config.Mode == "development" {
		logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	} else {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}
	gob.Register(Flash{})
	var templateFunc = template.FuncMap{
		"htmldate": func(in time.Time) string {
			return in.Format("2006-01-02")
		},
		"userdate": func(in time.Time) string {
			return in.Format("02.01.2006")
		},
		"timeago": func(in time.Time) string {
			return timeagoGerman.Format(in)
		},
		"taxtype": func(in string) string {
			taxtype := map[string]string{
				"S":  "Umsatzsteuerpflichtige Umsätze",
				"G":  "Ausfuhrlieferung (Außerhalb EU)",
				"K":  "Innergemeinschaftliche Lieferungen",
				"E":  "Steuerfreie Umsätze §4 UStG",
				"AE": "Reverse Charge",
			}
			if desc, ok := taxtype[in]; ok {
				return desc
			}
			return "unbekannt"
		},
		"rounddecimal": func(in decimal.Decimal) string {
			return in.Round(2).StringFixed(2)
		},
		"invoiceStatus": func(in model.InvoiceStatus) string {
			status := map[model.InvoiceStatus]string{
				model.InvoiceStatusDraft:  "Entwurf",
				model.InvoiceStatusIssued: "Offen",
				model.InvoiceStatusPaid:   "Bezahlt",
				model.InvoiceStatusVoided: "Storniert",
			}
			if desc, ok := status[in]; ok {
				return desc
			}
			return "unbekannt"
		},
		"unittype": func(in string) string {
			unittype := map[string]string{
				"C62": "Stück",
				"LS":  "pauschal",
				"HUR": "Stunden",
				"DAY": "Tage",
				"WEE": "Wochen",
				"MON": "Monate",
			}
			if desc, ok := unittype[in]; ok {
				return desc
			}
			return "unbekannt"
		},
		"array": func(els ...any) []any {
			return els
		},
		"toJSON": func(v any) template.JS {
			b, _ := json.Marshal(v)
			return template.JS(b)
		},
		"fmtTime": func(t time.Time) string {
			if t.IsZero() {
				return ""
			}
			return t.Format("02.01.2006 15:04")
		},
		"nl2br": func(s string) template.HTML {
			esc := html.EscapeString(s)
			return template.HTML(strings.ReplaceAll(esc, "\n", "<br>"))
		},
		"splitCSV": func(s string) []string {
			parts := strings.Split(s, ",")
			out := make([]string, 0, len(parts))
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					out = append(out, p)
				}
			}
			return out
		},
		"now":    time.Now,
		"before": func(a, b time.Time) bool { return a.Before(b) },
		"isOpen": func(s model.InvoiceStatus) bool {
			return s == "open" || s == "issued"
		}}

	tmpl := &Template{
		templates: template.Must(template.New("t").Funcs(templateFunc).ParseGlob("public/views/*.html")),
	}

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Pre(middleware.MethodOverrideWithConfig(middleware.MethodOverrideConfig{
		Getter: middleware.MethodFromForm("_method"),
	}))
	e.Pre(middleware.RemoveTrailingSlash())
	e.Use(middleware.BodyLimit("20M"))
	e.Use(middleware.RequestID()) // adds X-Request-ID
	e.Use(middleware.RecoverWithConfig(middleware.RecoverConfig{
		DisableStackAll:   false, // only log stack trace
		DisablePrintStack: true,
	}))

	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			// Request-ID (setzt vorher z.B. echo/middleware.RequestID)
			req := c.Request()
			res := c.Response()
			rid := res.Header().Get(echo.HeaderXRequestID)

			// Request-scoped Logger bauen und in den Context legen
			reqLogger := slog.With(
				"request_id", rid,
			).WithGroup("http").With(
				"method", req.Method,
				"path", req.URL.Path,
				"remote_ip", c.RealIP(),
			)
			c.Set("logger", reqLogger)

			// Handler ausführen
			err := next(c)

			if shouldSkipAccessLog(c) {
				return err
			}
			latency := time.Since(start)

			attrs := []any{
				"status", res.Status,
				"latency_ms", float64(latency.Microseconds()) / 1000.0,
			}

			// Level anhand Status wählen – benutze den request-scoped Logger
			switch {
			case res.Status >= 500:
				reqLogger.Error("http_request", attrs...)
			case res.Status >= 400:
				reqLogger.Warn("http_request", attrs...)
			default:
				reqLogger.Info("http_request", attrs...)
			}
			return err
		}
	})

	// Eigene HTTPErrorHandler: intern alles loggen, extern nur sichere Payload
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		l, _ := c.Get("logger").(*slog.Logger)
		if l == nil {
			l = logger
		}

		var ae *appError
		var he *echo.HTTPError
		switch {
		case errors.As(err, &ae):
			// schon unsere appError
		case errors.As(err, &he):
			// Nur 4xx-Mitteilungen an Nutzer durchlassen; 5xx maskieren
			public := ""
			if he.Code >= 400 && he.Code < 500 {
				public = fmt.Sprint(he.Message) // sicherer, explizit gesetzter Text
			}
			// mappe auf App-Fehler; in Err landet die Roh-Nachricht (nur fürs Log)
			ae = &appError{
				Code:   httpStatusToCode(he.Code), // helper s.u.
				Status: he.Code,
				Err:    fmt.Errorf("%v", he.Message),
				Public: public,
			}
		case errors.Is(err, echo.ErrNotFound):
			ae = ErrNotFound(err)
		case errors.Is(err, echo.ErrMethodNotAllowed):
			ae = &appError{Code: "METHOD_NOT_ALLOWED", Status: http.StatusMethodNotAllowed, Err: err}
		default:
			ae = ErrInternal(err)
		}

		attrs := []any{
			"status", ae.Status, // ursprünglicher HTTP-Status (z.B. 400)
			"code", ae.Code,
			"error", ae.Err.Error(),
		}
		if ae.Status >= 500 {
			l.Error("handler_error", attrs...)
		} else {
			l.Warn("handler_error", attrs...)
		}

		// HTML vs. JSON wie zuvor:
		if wantsHTML(c.Request()) {
			kind := "error"
			if ae.Status >= 400 && ae.Status < 500 {
				kind = "warning"
			}
			if err = AddFlash(c, kind, userMessage(ae)); err != nil {
				// Nur Loggen, weil wir eh schon im Fehler sind
				l.Error("cannot add flash message", "error", err)
			}
			// Redirect (Referer oder Fallback)
			target := c.Request().Referer()
			if target == "" {
				target = "/"
			}
			_ = c.Redirect(http.StatusSeeOther, target)
			return
		}

		_ = c.JSON(ae.Status, map[string]any{
			"error":      userMessage(ae),
			"error_code": ae.Code,
			"request_id": c.Response().Header().Get(echo.HeaderXRequestID),
		})
	}

	store := sessions.NewCookieStore([]byte(crmdb.Config.CookieSecret))
	store.Options = &sessions.Options{
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		// Secure: true, // in PROD mit HTTPS aktivieren
	}
	e.Use(session.Middleware(store))
	e.Use(FlashLoader)
	// irgendwo in NewController, NUR in dev:
	if crmdb.Config.Mode == "development" {
		// Disable caching for static files
		e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				if strings.HasPrefix(c.Request().URL.Path, "/static/") {
					res := c.Response().Header()
					res.Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
					res.Set("Pragma", "no-cache")
					res.Set("Expires", "0")
				}
				return next(c)
			}
		})
		e.GET("/__dev/flash", func(c echo.Context) error {
			kind := c.QueryParam("k")
			if kind == "" {
				kind = "info"
			}
			msg := c.QueryParam("m")
			if msg == "" {
				msg = "Demo-Flash aus __dev/flash"
			}
			to := c.QueryParam("to")
			if to == "" {
				to = "/"
			}

			if err := AddFlash(c, kind, msg); err != nil {
				return err
			}
			return c.Redirect(http.StatusFound, to)
		})
	}
	e.Use(middleware.CSRFWithConfig(middleware.CSRFConfig{
		TokenLength:    32,
		TokenLookup:    "form:csrf,header:X-CSRF-Token", // wo Echo den Token erwartet
		CookieName:     "csrf",                          // Name des CSRF-Cookies
		CookiePath:     "/",
		CookieHTTPOnly: true,
		CookieSameSite: http.SameSiteLaxMode,
		// CookieSecure: true, // in PROD mit HTTPS aktivieren
		Skipper: func(c echo.Context) bool {
			if c.Request().Method == http.MethodPost {
				if strings.HasPrefix(c.Path(), "/passwordreset") {
					return true
				}
				if strings.HasPrefix(c.Path(), "/login") {
					return true
				}
			}
			return false
		},
	}))

	e.Renderer = tmpl
	ctrl := controller{model: crmdb}
	e.GET("/", ctrl.root, ctrl.authMiddleware)
	e.GET("/search", ctrl.search, ctrl.authMiddleware)
	e.GET("/login", ctrl.login)
	e.POST("/login", ctrl.login)
	e.GET("/logout", ctrl.logout)
	e.GET("/register", ctrl.register)
	e.POST("/register", ctrl.register)
	e.GET("/verify", ctrl.verifyEmail)
	e.GET("/set-password", ctrl.showSetPasswordForm)
	e.POST("/set-password", ctrl.handleSetPasswordSubmit)
	e.GET("/passwordreset/:token", ctrl.showPasswordResetForm)
	e.POST("/passwordreset/:token", ctrl.handlePasswordResetSubmit)
	e.GET("/passwordreset", ctrl.showPasswordResetRequest)
	e.POST("/passwordreset", ctrl.handlePasswordResetRequest)

	e.Static("/static", "static")
	ctrl.invoiceInit(e)
	ctrl.companyInit(e)
	ctrl.personInit(e)
	ctrl.settingsInit(e)
	ctrl.fileManagerInit(e)
	ctrl.noteInit(e)
	ctrl.adminInit(e)
	ctrl.apiInit(e)

	if err := e.Start(fmt.Sprintf(":%d", crmdb.Config.Port)); err != nil {
		return fmt.Errorf("cannot start application %w", err)
	}
	return nil
}

func userMessage(ae *appError) string {
	if ae.Public != "" {
		return ae.Public
	}
	switch ae.Code {
	case "INVALID_INPUT":
		return "Die Eingabe ist ungültig. Bitte prüfen und erneut senden."
	case "NOT_FOUND":
		return "Die angeforderte Ressource wurde nicht gefunden."
	case "METHOD_NOT_ALLOWED":
		return "Diese HTTP-Methode wird hier nicht unterstützt."
	default:
		return "Es ist ein Fehler aufgetreten. Bitte später erneut versuchen."
	}
}

// kleine Helfer
func wantsHTML(r *http.Request) bool { return strings.Contains(r.Header.Get("Accept"), "text/html") }

func httpStatusToCode(status int) string {
	switch status {
	case 400:
		return "INVALID_INPUT"
	case 401:
		return "UNAUTHORIZED"
	case 403:
		return "FORBIDDEN"
	case 404:
		return "NOT_FOUND"
	case 405:
		return "METHOD_NOT_ALLOWED"
	default:
		if status >= 500 {
			return "INTERNAL"
		}
		return "ERROR"
	}
}

func shouldSkipAccessLog(c echo.Context) bool {
	p := c.Request().URL.Path
	if strings.HasPrefix(p, "/static/") || strings.HasPrefix(p, "/assets/") {
		return true
	}
	switch p {
	case "/favicon.ico", "/robots.txt", "/metrics":
		return true
	}
	// Optional: nach Dateiendungen filtern
	ext := strings.ToLower(path.Ext(p))
	switch ext {
	case ".css", ".js", ".map", ".png", ".jpg", ".jpeg", ".svg", ".ico", ".webp":
		return true
	}
	// Optional: HEAD/OPTIONS ausblenden
	m := c.Request().Method
	if m == http.MethodHead || m == http.MethodOptions {
		return true
	}
	return false
}
