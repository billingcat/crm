package controller

import (
	"net/http"
	"strings"

	"github.com/billingcat/crm/model"

	"github.com/labstack/echo/v4"
)

type settings struct {
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
	Uselocalcounter bool   `form:"uselocalcounter"`
	Bankname        string `form:"bankname"`
	Bankiban        string `form:"bankiban"`
	Bankbic         string `form:"bankbic"`
}

func (ctrl *controller) settingsInit(e *echo.Echo) {
	g := e.Group("/settings")
	g.Use(ctrl.authMiddleware)
	g.GET("/profile", ctrl.showProfile)
	g.POST("/profile", ctrl.updateProfile)
	g.GET("", ctrl.settingslist)
	g.POST("", ctrl.settingslist)
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
		cp := new(settings)
		if err := c.Bind(cp); err != nil {
			return ErrInvalid(err, "Fehler beim Verarbeiten der Formulardaten")
		}
		dbSettings := &model.Settings{
			InvoiceContact:        cp.Contactperson,
			InvoiceEMail:          cp.Ownemail,
			ZIP:                   cp.ZIP,
			Address1:              cp.Address1,
			Address2:              cp.Address2,
			City:                  cp.City,
			CountryCode:           cp.CountryCode,
			VATID:                 cp.VAT,
			TAXNumber:             cp.TaxNo,
			InvoiceNumberTemplate: cp.Invoicetemplate,
			UseLocalCounter:       cp.Uselocalcounter,
			BankIBAN:              cp.Bankiban,
			BankName:              cp.Bankname,
			BankBIC:               cp.Bankbic,
			CompanyName:           cp.Companyname,
		}
		dbSettings.ID = 1

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
	m := ctrl.defaultResponseMap(c, "Profile")
	m["user"] = u
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
