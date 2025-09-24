package controller

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"github.com/billingcat/crm/model"
)

type CookieCfg struct {
	IsProd       bool   // true in Production
	ShareSubdoms bool   // true, wenn Cookie über mehrere Subdomains gehen soll
	ParentDomain string // "billingcat.de" (nur nötig, wenn ShareSubdoms = true)
}

func cookieOptions(maxAge int, cfg CookieCfg) *sessions.Options {
	opts := &sessions.Options{
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		// Default: sicher & bequem
		SameSite: http.SameSiteLaxMode,
	}

	if cfg.IsProd {
		opts.Secure = true // https://app.billingcat.de -> MUSS true sein
		// Domain nur setzen, wenn du wirklich Subdomains teilen willst:
		if cfg.ShareSubdoms && cfg.ParentDomain != "" {
			opts.Domain = "." + cfg.ParentDomain // z.B. ".billingcat.de"
		}
	} else {
		// Local dev auf http://localhost
		opts.Secure = false // sonst wird Cookie nicht gesetzt
		// Domain weglassen (Host-only -> "localhost")
	}

	return opts
}

// authMiddleware is a middleware that checks if the user is authenticated.
func (ctrl *controller) authMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		sess, err := session.Get("session", c)
		if err != nil {
			return echo.NewHTTPError(500, fmt.Errorf("cannot get session %w", err))
		}
		uid := sess.Values["uid"]
		if uid, ok := uid.(uint); ok {
			c.Set("uid", uid)
		} else {
			return c.Redirect(http.StatusSeeOther, "/login")
		}
		ownerid := sess.Values["ownerid"]
		if ownerid, ok := ownerid.(uint); ok {
			c.Set("ownerid", ownerid)
		} else {
			return c.Redirect(http.StatusSeeOther, "/login")
		}
		return next(c)
	}
}

func (ctrl *controller) login(c echo.Context) error {
	if c.Request().Method == http.MethodGet {
		m := ctrl.defaultResponseMap(c, "Login")
		return c.Render(http.StatusOK, "login.html", m)
	}

	email := strings.TrimSpace(strings.ToLower(c.FormValue("email")))
	password := c.FormValue("password")
	remember := c.FormValue("rememberMe") != ""

	user, err := ctrl.model.AuthenticateUser(email, password)
	if err != nil || user == nil {
		if err = AddFlash(c, "error", "Login fehlgeschlagen. Bitte prüfe die Eingaben."); err != nil {
			return ErrInvalid(err, "Fehler beim Speichern der Session")
		}
		return c.Redirect(http.StatusSeeOther, "/login")
	}

	sess, err := session.Get("session", c)
	if err != nil {
		return echo.NewHTTPError(500, err)
	}

	maxAge := 0
	if remember {
		maxAge = 60 * 60 * 24 * 365 // 1 Jahr
	}

	// Prod/Local abhängig setzen
	opts := cookieOptions(maxAge, CookieCfg{
		IsProd:       ctrl.model.Config.Mode == "production",
		ShareSubdoms: false,           // nur app.billingcat.de -> false
		ParentDomain: "billingcat.de", // nur relevant, wenn ShareSubdoms=true
	})

	// Wenn du OAuth gegen fremde Domains nutzt und dabei das Session-Cookie
	// über Cross-Site-Redirects gesendet werden MUSS:
	// opts.SameSite = http.SameSiteNoneMode // NUR in Prod mit opts.Secure = true!

	sess.Options = opts

	// uid ist auch die owner id, das könnte sich aber ändern, wenn es mal Teams o.ä. gibt
	// (dann OwnerID in Session speichern)
	sess.Values["uid"] = user.ID
	sess.Values["ownerid"] = user.ID

	if err := sess.Save(c.Request(), c.Response()); err != nil {
		return echo.NewHTTPError(500, err)
	}
	return c.Redirect(http.StatusSeeOther, "/")
}

func (ctrl *controller) logout(c echo.Context) error {
	sess, err := session.Get("session", c)
	if err != nil {
		return echo.NewHTTPError(500, err)
	}
	delete(sess.Values, "uid")
	delete(sess.Values, "ownerid")
	delete(sess.Values, "csrf")

	// Cookie wirklich löschen (Safari!)
	if sess.Options == nil {
		sess.Options = &sessions.Options{Path: "/"}
	}
	sess.Options.MaxAge = -1

	if err := sess.Save(c.Request(), c.Response()); err != nil {
		return echo.NewHTTPError(500, err)
	}
	AddFlash(c, "success", "Du wurdest abgemeldet.")
	return c.Redirect(http.StatusFound, "/login")
}

func generateResetToken() (token string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil { //
		return "", nil, err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(token))
	return token, h[:], nil
}

func constantTimeMatchToken(providedToken string, storedHash []byte) bool {
	sum := sha256.Sum256([]byte(providedToken))
	// gleicher Length-Check + constant-time compare
	return len(storedHash) == len(sum[:]) && hmac.Equal(storedHash, sum[:])
}

// 1) Reset anfordern (GET Formular)
func (ctrl *controller) showPasswordResetRequest(c echo.Context) error {
	m := ctrl.defaultResponseMap(c, "Password Reset")
	return c.Render(http.StatusOK, "passwordreset.html", m)
}

// 1) Reset anfordern (POST)
func (ctrl *controller) handlePasswordResetRequest(c echo.Context) error {
	logger := c.Get("logger").(*slog.Logger)
	email := strings.TrimSpace(strings.ToLower(c.FormValue("email")))

	// Immer gleich reagieren – egal, ob der User existiert
	genericResponse := func() error {
		AddFlash(c, "info", "Wenn ein Konto existiert, haben wir dir eine E-Mail geschickt.")
		return c.Redirect(http.StatusSeeOther, "/login")
	}

	user, err := ctrl.model.GetUserByEMail(email)
	if err != nil || user == nil {
		// absichtlich kein Unterschied nach außen
		return genericResponse()
	}

	token, tokenHash, err := generateResetToken()
	if err != nil {
		// trotzdem generisch antworten
		logger.Error("cannot generate reset token", "error", err)
		return genericResponse()
	}

	// Setze Hash + Expiry (z. B. 1h)
	user.PasswordResetToken = tokenHash
	user.PasswordResetExpiry = time.Now().UTC().Add(1 * time.Hour)
	if err := ctrl.model.UpdateUser(user); err != nil {
		logger.Error("cannot store reset token", "error", err)
		return genericResponse()
	}
	resetURL := url.URL{
		Scheme: c.Scheme(),
		Host:   c.Request().Host,
		Path:   c.Request().RequestURI + "/" + url.PathEscape(token),
	}

	body := fmt.Sprintf("Bitte klicke auf den Link, um dein Passwort zurückzusetzen:\n\n%s\n\nDer Link ist 60 Minuten gültig.", resetURL.String())
	_ = ctrl.sendEmail(email, "Passwort zurücksetzen", body)

	return genericResponse()
}

// 2) Neues Passwort setzen – Formular anzeigen (Token prüfen)
func (ctrl *controller) showPasswordResetForm(c echo.Context) error {
	token := c.Param("token")

	sum := sha256.Sum256([]byte(token))
	user, err := ctrl.model.GetUserByResetTokenHashPrefix(sum[:], 16) // oder gezielt Lookup by hash
	if err != nil || user == nil || user.PasswordResetExpiry.Before(time.Now().UTC()) ||
		!constantTimeMatchToken(token, user.PasswordResetToken) {
		// generisch bleiben
		AddFlash(c, "error", "Der Link ist ungültig oder abgelaufen.")
		return c.Redirect(http.StatusSeeOther, "/login")
	}

	m := ctrl.defaultResponseMap(c, "Passwort neu setzen")
	m["token"] = token
	return c.Render(http.StatusOK, "passwordresettoken.html", m)
}

// 2) Neues Passwort setzen – Submit
func (ctrl *controller) handlePasswordResetSubmit(c echo.Context) error {
	token := c.Param("token")
	pass := c.FormValue("newPassword")
	confirm := c.FormValue("confirmPassword")
	logger := c.Get("logger").(*slog.Logger)

	// Validierung
	if pass == "" || pass != confirm {
		_ = AddFlash(c, "error", "Bitte prüfe die Eingaben (Passwörter stimmen nicht überein).")
		return c.Redirect(http.StatusSeeOther, c.Request().RequestURI)
	}

	sum := sha256.Sum256([]byte(token))
	user, err := ctrl.model.GetUserByResetTokenHashPrefix(sum[:], 16)
	if err != nil || user == nil || user.PasswordResetExpiry.Before(time.Now().UTC()) ||
		!constantTimeMatchToken(token, user.PasswordResetToken) {
		_ = AddFlash(c, "error", "Der Link ist ungültig oder abgelaufen.")
		return c.Redirect(http.StatusSeeOther, "/login")
	}

	// Passwort setzen (z. B. bcrypt/argon2id in SetPassword)
	if err := ctrl.model.SetPassword(user, pass); err != nil {
		logger.Error("cannot set password", "error", err)
		_ = AddFlash(c, "error", "Interner Fehler. Bitte später erneut versuchen.")
		return c.Redirect(http.StatusSeeOther, "/login")
	}
	// Token ungültig machen
	user.PasswordResetToken = nil
	user.PasswordResetExpiry = time.Time{}
	if err := ctrl.model.UpdateUser(user); err != nil {
		logger.Error("cannot clear reset token", "error", err)
		// Passwort ist bereits gesetzt; notfalls trotzdem Erfolg anzeigen
	}
	_ = AddFlash(c, "success", "Dein Passwort wurde zurückgesetzt. Du kannst dich jetzt anmelden.")
	return c.Redirect(http.StatusSeeOther, "/login")
}

func (ctrl *controller) register(c echo.Context) error {
	if !ctrl.model.Config.RegistrationAllowed {
		return echo.NewHTTPError(403, "Registration is disabled")
	}
	if c.Request().Method == http.MethodGet {
		m := ctrl.defaultResponseMap(c, "Register")
		return c.Render(http.StatusOK, "register.html", m)
	}

	email := strings.TrimSpace(strings.ToLower(c.FormValue("email")))
	password := c.FormValue("password")

	// Check if user already exists
	existingUser, err := ctrl.model.GetUserByEMail(email)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return echo.NewHTTPError(500, fmt.Errorf("database error: %w", err))
	}
	if existingUser != nil {
		return echo.NewHTTPError(400, "user already exists")
	}

	// Create new user
	user := &model.User{
		Email: email,
	}
	if err := ctrl.model.SetPassword(user, password); err != nil {
		return echo.NewHTTPError(500, fmt.Errorf("error setting password: %w", err))
	}
	if err := ctrl.model.CreateUser(user); err != nil {
		return echo.NewHTTPError(500, fmt.Errorf("error saving user: %w", err))
	}

	return c.Redirect(http.StatusSeeOther, "/login")
}
