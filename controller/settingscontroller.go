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

// settingsForm mirrors the profile/settings HTML form fields.
// Names are kept to match the form payload; values are bound via Echo.
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
	Uselocalcounter bool   `form:"uselocalcounter"` // comes as "true"/"false"
	Bankname        string `form:"bankname"`
	Bankiban        string `form:"bankiban"`
	Bankbic         string `form:"bankbic"`
	CustomerPrefix  string `form:"custprefix"`  // e.g. "K-"
	CustomerWidth   int    `form:"custwidth"`   // e.g. 5
	CustomerCounter int64  `form:"custcounter"` // e.g. 1000

}

func (ctrl *controller) settingsInit(e *echo.Echo) {
	g := e.Group("/settings")
	g.Use(ctrl.authMiddleware)
	g.GET("/profile", ctrl.showProfile)
	g.POST("/profile", ctrl.updateProfile)
	g.POST("/profile/delete-start", ctrl.settingsDeleteStart)    // validates "DELETE", then redirect
	g.GET("/profile/delete-confirm", ctrl.settingsDeleteConfirm) // show password confirm page
	g.POST("/profile/delete-confirm", ctrl.settingsDeleteDo)     // verify password, soft-delete
	g.GET("/goodbye", ctrl.goodbye)                              // optional farewell page
	g.POST("/tokens/create", ctrl.settingsTokenCreate)           // create a new API token
	g.GET("/tokens/create", ctrl.settingsTokenCreate)
	g.POST("/tokens/revoke/:id", ctrl.settingsTokenRevoke) // revoke an existing token
	g.GET("", ctrl.settingslist)
	g.POST("", ctrl.settingslist)
}

// controller/views.go
// ProfilePageData is the template view model for the profile page.
type ProfilePageData struct {
	CSRFToken string
	User      *model.User
	Tokens    []model.APIToken
	NewToken  string // set only when a new plaintext token was just created
}

// settingslist renders and processes the tenant-level settings form.
// GET: load settings; POST: upsert settings and redirect to home.
func (ctrl *controller) settingslist(c echo.Context) error {
	m := ctrl.defaultResponseMap(c, "Settings")
	m["action"] = "/settings"
	m["submit"] = "Save"
	m["cancel"] = "/"
	ownerID := c.Get("ownerid").(uint)

	switch c.Request().Method {
	case http.MethodGet:
		settings, err := ctrl.model.LoadSettings(ownerID)
		if err != nil {
			return ErrInvalid(err, "Error loading settings")
		}
		m["settings"] = settings
		return c.Render(http.StatusOK, "settingslist.html", m)

	case http.MethodPost:
		f := new(settingsForm)
		if err := c.Bind(f); err != nil {
			c.Get("logger").(*slog.Logger).Error("binding settings form failed", "err", err)
			return ErrInvalid(err, "Error processing form data")
		}

		dbSettings := &model.Settings{
			OwnerID:               ownerID,
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
			CustomerNumberPrefix:  f.CustomerPrefix,
			CustomerNumberWidth:   f.CustomerWidth,
			CustomerNumberCounter: f.CustomerCounter,
		}

		if err := ctrl.model.SaveSettings(dbSettings); err != nil {
			return ErrInvalid(err, "Error saving settings")
		}

		return c.Redirect(http.StatusSeeOther, "/")
	}
	return nil
}

// showProfile renders the user profile page, including the list of API tokens
// belonging to the user's owner/tenant.
func (ctrl *controller) showProfile(c echo.Context) error {
	uid := c.Get("uid").(uint)

	u, err := ctrl.model.GetUserByID(uid)
	if err != nil || u == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "cannot load profile")
	}

	// Load tokens for the owner
	tokens, _, err := ctrl.model.ListAPITokensByOwner(u.OwnerID, 100, "")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "cannot load api tokens")
	}

	m := ctrl.defaultResponseMap(c, "Profile")
	m["user"] = u
	m["tokens"] = tokens
	// m["newToken"] may optionally be set by the create handler
	return c.Render(http.StatusOK, "profile.html", m)
}

// updateProfile updates simple user profile fields (currently only FullName).
func (ctrl *controller) updateProfile(c echo.Context) error {
	uid := c.Get("uid").(uint)
	u, err := ctrl.model.GetUserByID(uid)
	if err != nil || u == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "cannot load profile")
	}

	full := strings.TrimSpace(c.FormValue("fullname"))
	u.FullName = full

	if err := ctrl.model.UpdateUser(u); err != nil {
		_ = AddFlash(c, "error", "Could not save changes.")
		return c.Redirect(http.StatusSeeOther, "/settings/profile")
	}
	_ = AddFlash(c, "success", "Profile saved.")
	return c.Redirect(http.StatusSeeOther, "/settings/profile")
}

// settingsTokenCreate creates a new API token for the current user’s owner.
// Returns the plaintext token directly on the profile page (no redirect),
// because it can only be shown once.
func (ctrl *controller) settingsTokenCreate(c echo.Context) error {
	uid := c.Get("uid").(uint)

	// if method == GET, show settings page
	if c.Request().Method == http.MethodGet {
		return ctrl.showProfile(c)
	}
	u, err := ctrl.model.GetUserByID(uid)
	if err != nil || u == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "cannot load user")
	}

	name := strings.TrimSpace(c.FormValue("name"))
	// MVP: no scopes/expiry yet
	var expiresAt *time.Time
	plain, _, err := ctrl.model.CreateAPIToken(u.OwnerID, &u.ID, name, "", expiresAt)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "cannot create api token")
	}

	// Reload list for display
	tokens, _, err := ctrl.model.ListAPITokensByOwner(u.OwnerID, 100, "")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "cannot load api tokens")
	}

	// Important: no redirect — show the plaintext token immediately
	m := ctrl.defaultResponseMap(c, "Profile")
	m["user"] = u
	m["tokens"] = tokens
	m["newToken"] = plain // shown once in the template
	return c.Render(http.StatusOK, "profile.html", m)
}

// settingsTokenRevoke revokes (disables) a token for the current user's owner.
// Redirects back to the profile page after success.
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

	// Redirect back to profile (safe — no plaintext token involved here)
	return c.Redirect(http.StatusFound, "/settings/profile")
}

// settingsDeleteStart validates the "DELETE" confirmation and redirects to the password confirm page.
func (ctrl *controller) settingsDeleteStart(c echo.Context) error {
	confirm := c.FormValue("confirm")
	if confirm != "DELETE" {
		AddFlash(c, "error", "Bestätigung fehlgeschlagen. Tippe exakt „DELETE“.")
		return c.Redirect(http.StatusSeeOther, "/settings/profile")
	}

	// Optional: set a short-lived flag in session to allow the confirm page (anti-CSRF/flow hardening).
	if err := SetSessionValue(c, "delete_flow_ok", "1"); err != nil {
		c.Get("logger").(*slog.Logger).Warn("sessionSet delete_flow_ok failed", "err", err)
	}

	return c.Redirect(http.StatusSeeOther, "/settings/profile/delete-confirm")
}

// settingsDeleteConfirm renders the password prompt page.
func (ctrl *controller) settingsDeleteConfirm(c echo.Context) error {
	// Optional: require the flow flag
	if v := GetSessionValue(c, "delete_flow_ok"); v != "1" {
		AddFlash(c, "error", "Ungültiger Lösch-Workflow. Starte erneut.")
		return c.Redirect(http.StatusSeeOther, "/settings/profile")
	}
	return c.Render(http.StatusOK, "delete_confirm.html", ctrl.defaultResponseMap(c, "Löschung bestätigen"))
}

// settingsDeleteDo verifies the password and performs soft-delete + immediate access revocation.
func (ctrl *controller) settingsDeleteDo(c echo.Context) error {
	// CSRF via hidden input is already present

	// Require correct flow
	if v := GetSessionValue(c, "delete_flow_ok"); v != "1" {
		AddFlash(c, "error", "Ungültiger Lösch-Workflow. Starte erneut.")
		return c.Redirect(http.StatusSeeOther, "/settings/profile")
	}
	// Clear the flow flag
	_ = DeleteSessionValue(c, "delete_flow_ok")

	// Verify password
	password := c.FormValue("password")
	if password == "" {
		AddFlash(c, "error", "Bitte Passwort eingeben.")
		return c.Redirect(http.StatusSeeOther, "/settings/profile/delete-confirm")
	}
	userID := c.Get("uid").(uint)
	user, err := ctrl.model.GetUserByID(userID)
	if err != nil || user == nil {
		c.Get("logger").(*slog.Logger).Error("cannot load user for delete", "err", err, "userID", userID)
		AddFlash(c, "error", "Interner Fehler.")
		return c.Redirect(http.StatusSeeOther, "/settings/profile/delete-confirm")
	}

	if !ctrl.model.CheckPassword(user, password) {
		AddFlash(c, "error", "Passwort falsch.")
		return c.Redirect(http.StatusSeeOther, "/settings/profile/delete-confirm")
	}

	// 1) Immediately revoke access (tokens + sessions)
	if err := ctrl.model.RevokeUserAccessImmediate(c.Request().Context(), userID); err != nil {
		c.Get("logger").(*slog.Logger).Error("RevokeUserAccessImmediate failed", "err", err, "userID", userID)
		AddFlash(c, "error", "Fehler beim Widerrufen der Zugänge.")
		return c.Redirect(http.StatusSeeOther, "/settings/profile/delete-confirm")
	}

	// 2) Soft-delete the user account (mark as deleted_at, start 30-day grace)
	if err := ctrl.model.SoftDeleteUserAccount(c.Request().Context(), userID); err != nil {
		c.Get("logger").(*slog.Logger).Error("SoftDeleteUserAccount failed", "err", err, "userID", userID)
		AddFlash(c, "error", "Account konnte nicht gelöscht werden.")
		return c.Redirect(http.StatusSeeOther, "/settings/profile/delete-confirm")
	}

	// 3) Logout current session (cookie/session invalidation)
	if err := ctrl.logout(c); err != nil {
		c.Get("logger").(*slog.Logger).Warn("logout failed after delete", "err", err)
	}

	return c.Redirect(http.StatusSeeOther, "/goodbye")
}

// Optional minimal goodbye page
func (ctrl *controller) goodbye(c echo.Context) error {
	return c.Render(http.StatusOK, "goodbye.html", map[string]any{
		"Title":   "Account gelöscht",
		"Message": "Dein Account wurde zur Löschung vorgemerkt. Zugänge sind widerrufen; Datenlöschung erfolgt routinemäßig nach 30 Tagen.",
	})
}
