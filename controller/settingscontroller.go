package controller

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/billingcat/crm/model"

	"github.com/labstack/echo/v4"
)

type settingsForm struct {
	Companyname     string `form:"companyname"`
	Contactperson   string `form:"contactperson"`
	Ownemail        string `form:"ownemail"`
	Address1        string `form:"address1"`
	Address2        string `form:"address2"`
	ZIP             string `form:"zip"`
	City            string `form:"city"`
	CountryCode     string `form:"countrycode"`
	VAT             string `form:"vat"`
	TaxNo           string `form:"taxno"`
	Invoicetemplate string `form:"invoicetemplate"`
	Uselocalcounter bool   `form:"uselocalcounter"` // kommt als "true"/"false"
	Bankname        string `form:"bankname"`
	Bankiban        string `form:"bankiban"`
	Bankbic         string `form:"bankbic"`
}

func (ctrl *controller) settingsInit(e *echo.Echo) {
	g := e.Group("/settings")
	g.Use(ctrl.authMiddleware)
	g.GET("/profile", ctrl.showProfile)
	g.POST("/profile", ctrl.updateProfile)
	g.POST("/tokens/create", ctrl.settingsTokenCreate)     // erstellt einen neuen Token
	g.POST("/tokens/revoke/:id", ctrl.settingsTokenRevoke) // sperrt (revoked) einen Token

	g.GET("", ctrl.settingslist)
	g.POST("", ctrl.settingslist)
}

// controller/views.go
type ProfilePageData struct {
	CSRFToken string
	User      *model.User
	Tokens    []model.APIToken
	NewToken  string // nur gesetzt direkt nach dem Erstellen
}

func (ctrl *controller) settingslist(c echo.Context) error {
	m := ctrl.defaultResponseMap(c, "Einstellungen")
	m["action"] = "/settings"
	m["submit"] = "Speichern"
	m["cancel"] = "/"
	ownerID := c.Get("ownerid")
	switch c.Request().Method {
	case http.MethodGet:
		settings, err := ctrl.model.LoadSettings(ownerID)
		if err != nil {
			return ErrInvalid(err, "Fehler beim Laden der Einstellungen")
		}
		m["settings"] = settings
		return c.Render(http.StatusOK, "settingslist.html", m)
	case http.MethodPost:
		f := new(settingsForm)
		if err := c.Bind(f); err != nil {
			c.Get("logger").(*slog.Logger).Error("binding settings form failed", "err", err)
			return ErrInvalid(err, "Fehler beim Verarbeiten der Formulardaten")
		}
		oid := c.Get("ownerid").(uint) // wichtig: als uint

		dbSettings := &model.Settings{
			OwnerID:               oid,
			CompanyName:           f.Companyname,
			InvoiceContact:        f.Contactperson,
			InvoiceEMail:          f.Ownemail,
			Address1:              f.Address1,
			Address2:              f.Address2,
			ZIP:                   f.ZIP,
			City:                  f.City,
			CountryCode:           f.CountryCode,
			VATID:                 f.VAT,
			TAXNumber:             f.TaxNo,
			InvoiceNumberTemplate: f.Invoicetemplate,
			UseLocalCounter:       f.Uselocalcounter,
			BankName:              f.Bankname,
			BankIBAN:              f.Bankiban,
			BankBIC:               f.Bankbic,
		}

		if err := ctrl.model.SaveSettings(dbSettings); err != nil {
			return ErrInvalid(err, "Fehler beim Speichern der Einstellungen")
		}

		return c.Redirect(http.StatusSeeOther, "/")
	}
	return nil
}

func (ctrl *controller) showProfile(c echo.Context) error {
	uid := c.Get("uid").(uint)

	u, err := ctrl.model.GetUserByID(uid)
	if err != nil || u == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "cannot load profile")
	}

	// Tokens für den Owner laden
	tokens, _, err := ctrl.model.ListAPITokensByOwner(u.OwnerID, 100, "")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "cannot load api tokens")
	}

	m := ctrl.defaultResponseMap(c, "Profile")
	m["user"] = u
	m["tokens"] = tokens
	// m["newToken"] kann (optional) vom Create-Handler gesetzt werden
	return c.Render(http.StatusOK, "profile.html", m)
}

func (ctrl *controller) updateProfile(c echo.Context) error {
	uid := c.Get("uid").(uint)
	u, err := ctrl.model.GetUserByID(uid)
	if err != nil || u == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "cannot load profile")
	}

	full := strings.TrimSpace(c.FormValue("fullname"))
	u.FullName = full

	if err := ctrl.model.UpdateUser(u); err != nil {
		_ = AddFlash(c, "error", "Konnte die Daten nicht speichern.")
		return c.Redirect(http.StatusSeeOther, "/settings/profile")
	}
	_ = AddFlash(c, "success", "Profil gespeichert.")
	return c.Redirect(http.StatusSeeOther, "/settings/profile")
}

func (ctrl *controller) settingsTokenCreate(c echo.Context) error {
	uid := c.Get("uid").(uint)

	u, err := ctrl.model.GetUserByID(uid)
	if err != nil || u == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "cannot load user")
	}

	name := strings.TrimSpace(c.FormValue("name"))
	// MVP: keine Scopes/Ablaufzeit
	var expiresAt *time.Time
	plain, _, err := ctrl.model.CreateAPIToken(u.OwnerID, &u.ID, name, "", expiresAt)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "cannot create api token")
	}

	// Liste neu laden
	tokens, _, err := ctrl.model.ListAPITokensByOwner(u.OwnerID, 100, "")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "cannot load api tokens")
	}

	// Wichtig: kein Redirect – Klartext-Token direkt anzeigen
	m := ctrl.defaultResponseMap(c, "Profile")
	m["user"] = u
	m["tokens"] = tokens
	m["newToken"] = plain // <- im Template einmalig anzeigen
	return c.Render(http.StatusOK, "profile.html", m)
}

func (ctrl *controller) settingsTokenRevoke(c echo.Context) error {
	uid := c.Get("uid").(uint)

	u, err := ctrl.model.GetUserByID(uid)
	if err != nil || u == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "cannot load user")
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || id == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid token id")
	}

	if err := ctrl.model.RevokeAPIToken(u.OwnerID, uint(id)); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "cannot revoke token")
	}

	// zurück zum Profil (hier ist Redirect okay – kein Klartext nötig)
	return c.Redirect(http.StatusFound, "/settings/profile")
}
