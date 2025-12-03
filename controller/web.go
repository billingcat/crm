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

// FlashLoader pulls flash messages from the session (and clears them),
// stores them on the Echo context, and keeps remember-me intact by using SessionWriter.
func FlashLoader(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		sw, err := LoadSession(c)
		if err != nil {
			// Session not available; continue without flashes.
			c.Set("flashes", []Flash{})
			return next(c)
		}
		raw := sw.sess.Flashes() // consumes flashes
		// Save with proper cookie options so remember-me is preserved.
		_ = sw.Save()

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

// AddFlash stores a flash message in the session and preserves remember-me
// because SessionWriter.Save() reapplies cookie options on every save.
func AddFlash(c echo.Context, kind, msg string) error {
	sw, err := LoadSession(c)
	if err != nil {
		return ErrInvalid(err, "error loading session")
	}
	sw.AddFlash(Flash{Kind: kind, Message: msg})
	if err := sw.Save(); err != nil {
		return ErrInvalid(err, "error saving session")
	}
	return nil
}

type appError struct {
	Code   string // stable, internal error code for ops/support
	Status int    // mapped HTTP status
	Err    error  // original error (never exposed to clients)
	Public string // safe, optional user-facing message
}

func (e *appError) Error() string { return fmt.Sprintf("%s: %v", e.Code, e.Err) }
func (e *appError) Unwrap() error { return e.Err }

// Helpers to construct common app errors.
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
	// Configure "timeago" once (no max window, German strings for UI).
	timeagoGerman = timeago.NoMax(timeago.German)
)

// Template implements Echo's renderer interface.
type Template struct {
	templates *template.Template
}

// Render satisfies Echo's renderer interface.
func (t *Template) Render(w io.Writer, name string, data any, _ echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

type controller struct {
	model *model.Store
}

// defaultResponseMap builds a base map used by most views (title, flashes, auth info, etc.).
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

	// CSRF token (if middleware provided it on the context)
	if t := c.Get(middleware.DefaultCSRFConfig.ContextKey); t != nil {
		responseMap["CSRFToken"] = t.(string)
	}

	ownerID := c.Get("ownerid")
	userID := c.Get("uid")
	if ownerID == nil || userID == nil {
		return responseMap
	}

	// Admin flag passthrough
	if c.Get("is_admin") != nil {
		responseMap["is_admin"] = c.Get("is_admin").(bool)
	}
	responseMap["useInvitations"] = ctrl.model.Config.UseInvitationCodes
	responseMap["ownerid"] = ownerID
	responseMap["uid"] = userID.(uint)

	// Load minimal user info for header/menus
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

	// Recent items for sidebar/dashboard
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

// Safe HTML helpers for templates.
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

// root handles the dashboard/homepage.
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
				continue // note might have been deleted
			}

			// short, safe excerpt
			body := snippet(n.Body, 140)
			bodyEsc := escape(body)

			switch n.ParentType {
			case model.ParentTypeCompany:
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
						When: n.CreatedAt,
					})
				} else {
					// Fallback if company not available anymore
					changelog = append(changelog, lastChanges{
						Who: owner.FullName,
						What: template.HTML(fmt.Sprintf(
							`hat eine Notiz zu Firma #%d erstellt: <span class="text-slate-600 italic">%s</span>`,
							n.ParentID, bodyEsc,
						)),
						When: n.CreatedAt,
					})
				}

			case model.ParentTypePerson:
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
				// Unknown parent type (should not happen)
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

	// Heads are likely sorted already; enforce stability just in case.
	sort.SliceStable(changelog, func(i, j int) bool { return changelog[i].When.After(changelog[j].When) })
	if len(hydr.Companies) == 0 {
		m["nocompanies"] = true
	}
	m["lastchanges"] = changelog
	return c.Render(http.StatusOK, "main.html", m)
}

// search handles a small full-text search across companies and people.
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
		// simple string query
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
			Text:   company.Name,
			Action: fmt.Sprintf("/company/%d/%s", company.ID, url.PathEscape(company.Name)),
		})
	}

	for _, person := range people {
		searchResults = append(searchResults, searchResult{
			Text:   person.Name,
			Action: fmt.Sprintf("/person/%d/%s", person.ID, url.PathEscape(person.Name)),
		})
	}

	return c.JSON(http.StatusOK, searchResults)
}

// NewController wires routes, middleware, renderer, and starts the server.
func NewController(s *model.Store) error {
	// Environment-driven logger: Dev=Text+Debug, Prod=JSON+Info
	var logger *slog.Logger
	if s.Config.Mode == "development" {
		logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	} else {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}

	// Register types used in gorilla/sessions (e.g., Flash) to avoid gob errors.
	gob.Register(Flash{})
	ctrl := controller{model: s}

	// Template functions available in views.
	var templateFunc = template.FuncMap{
		"htmldate": func(in time.Time) string { return in.Format("2006-01-02") },
		"userdate": func(in time.Time) string { return in.Format("02.01.2006") },
		"timeago":  func(in time.Time) string { return timeagoGerman.Format(in) },
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
		"rounddecimal": func(in decimal.Decimal) string { return in.Round(2).StringFixed(2) },
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
		"array":  func(els ...any) []any { return els },
		"toJSON": func(v any) template.JS { b, _ := json.Marshal(v); return template.JS(b) },
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
		"htmlEscape": func(s string) string {
			return html.EscapeString(s)
		},
		"ceilDiv": func(a, b int64) int64 {
			if b <= 0 {
				return a
			}
			return (a + b - 1) / b
		},
		"tagsForParent": ctrl.tagsForParent,
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
		"isOpen": func(s model.InvoiceStatus) bool { return s == "open" || s == "issued" },
	}

	// Set up renderer
	tmpl := &Template{
		templates: template.Must(template.New("t").Funcs(templateFunc).ParseGlob("public/views/*.html")),
	}

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// --- Core middleware
	e.Pre(middleware.MethodOverrideWithConfig(middleware.MethodOverrideConfig{
		Getter: middleware.MethodFromForm("_method"),
	}))
	e.Pre(middleware.RemoveTrailingSlash())
	e.Use(middleware.BodyLimit("20M"))
	e.Use(middleware.RequestID()) // adds X-Request-ID
	e.Use(middleware.RecoverWithConfig(middleware.RecoverConfig{
		DisableStackAll:   false, // log stack trace only
		DisablePrintStack: true,
	}))

	// Request-scoped logger (adds structured attributes and unified access log).
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			req := c.Request()
			res := c.Response()
			rid := res.Header().Get(echo.HeaderXRequestID)

			reqLogger := slog.With(
				"request_id", rid,
			).WithGroup("http").With(
				"method", req.Method,
				"path", req.URL.Path,
				"remote_ip", c.RealIP(),
			)
			c.Set("logger", reqLogger)

			err := next(c)

			if shouldSkipAccessLog(c) {
				return err
			}
			latency := time.Since(start)
			attrs := []any{
				"status", res.Status,
				"latency_ms", float64(latency.Microseconds()) / 1000.0,
			}
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

	// Central HTTP error handler: log internally, show safe messages externally.
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		l, _ := c.Get("logger").(*slog.Logger)
		if l == nil {
			l = logger
		}

		var ae *appError
		var he *echo.HTTPError
		switch {
		case errors.As(err, &ae):
			// already an appError
		case errors.As(err, &he):
			// expose 4xx messages only; mask 5xx
			public := ""
			if he.Code >= 400 && he.Code < 500 {
				public = fmt.Sprint(he.Message)
			}
			ae = &appError{
				Code:   httpStatusToCode(he.Code),
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
			"status", ae.Status,
			"code", ae.Code,
			"error", ae.Err.Error(),
		}
		if ae.Status >= 500 {
			l.Error("handler_error", attrs...)
		} else {
			l.Warn("handler_error", attrs...)
		}

		// HTML vs JSON response
		if wantsHTML(c.Request()) {
			kind := "error"
			if ae.Status >= 400 && ae.Status < 500 {
				kind = "warning"
			}
			if err = AddFlash(c, kind, userMessage(ae)); err != nil {
				l.Error("cannot add flash message", "error", err)
			}
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

	// gorilla/sessions cookie store (client-side). Defaults are conservative;
	// remember-me MaxAge is controlled per-save by SessionWriter.
	store := sessions.NewCookieStore([]byte(s.Config.CookieSecret))
	e.Use(session.Middleware(store))
	store.Options = &sessions.Options{
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.Config.Mode == "production", // set Secure in prod (HTTPS)
		// Domain: set via CookieCfg if you share across subdomains
	}

	// Inject cookie config (used by SessionWriter to apply cookie options on save).
	e.Use(ctrl.CookieCfgMiddleware)

	// Flash loader must run after session middleware and before handlers.
	e.Use(FlashLoader)

	// In development, disable caching for static files and provide a flash demo route.
	if s.Config.Mode == "development" {
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

	// CSRF protection. Cookie is Lax and Secure in prod.
	e.Use(middleware.CSRFWithConfig(middleware.CSRFConfig{
		TokenLength:    32,
		TokenLookup:    "form:csrf,header:X-CSRF-Token",
		CookieName:     "csrf",
		CookiePath:     "/",
		CookieHTTPOnly: true,
		CookieSameSite: http.SameSiteLaxMode,
		CookieSecure:   s.Config.Mode == "production",
		Skipper: func(c echo.Context) bool {
			// allow POSTs to these endpoints without CSRF (e.g., public forms)
			if c.Request().Method == http.MethodPost {
				if strings.HasPrefix(c.Path(), "/password/reset") {
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

	// --- Routes
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

	e.GET("/password/reset/:token", ctrl.showPasswordResetForm)
	e.POST("/password/reset/:token", ctrl.handlePasswordResetSubmit)
	e.GET("/password/reset", ctrl.showPasswordResetRequest)
	e.POST("/password/reset", ctrl.handlePasswordResetRequest)

	e.Static("/static", "static")
	e.Static("/uploads", "uploads")
	// Feature modules
	ctrl.invoiceInit(e)
	ctrl.companyInit(e)
	ctrl.personInit(e)
	ctrl.tagsInit(e)
	ctrl.settingsInit(e)
	ctrl.fileManagerInit(e)
	ctrl.noteInit(e)
	ctrl.adminInit(e)
	ctrl.apiInit(e)
	ctrl.letterheadInit(e)
	ctrl.customernumberInit(e)

	if err := e.Start(fmt.Sprintf(":%d", s.Config.Port)); err != nil {
		return fmt.Errorf("cannot start application %w", err)
	}
	return nil
}

// userMessage maps an appError to a safe, German, user-facing message.
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

// wantsHTML returns true if the client accepts HTML.
func wantsHTML(r *http.Request) bool { return strings.Contains(r.Header.Get("Accept"), "text/html") }

// httpStatusToCode maps HTTP status codes to internal error codes.
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

// shouldSkipAccessLog filters out noise from the access log (static assets, etc.).
func shouldSkipAccessLog(c echo.Context) bool {
	p := c.Request().URL.Path
	if strings.HasPrefix(p, "/static/") || strings.HasPrefix(p, "/assets/") {
		return true
	}
	switch p {
	case "/favicon.ico", "/robots.txt", "/metrics":
		return true
	}
	// Optional: filter by file extensions
	ext := strings.ToLower(path.Ext(p))
	switch ext {
	case ".css", ".js", ".map", ".png", ".jpg", ".jpeg", ".svg", ".ico", ".webp":
		return true
	}
	// Optional: drop HEAD/OPTIONS
	m := c.Request().Method
	if m == http.MethodHead || m == http.MethodOptions {
		return true
	}
	return false
}
