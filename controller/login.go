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
)

// CookieCfg controls how the session cookie is scoped and secured.
// NOTE: Options are applied centrally by SessionWriter.Save() via applySessionOptionsFromPersist.
// This file only sets the "persist" flag (remember me) where needed.
type CookieCfg struct {
	IsProd       bool
	ShareSubdoms bool
	ParentDomain string
}

// cookieOptions builds secure cookie options based on environment.
// Kept for completeness if you need it elsewhere; SessionWriter uses this internally.
func cookieOptions(maxAge int, cfg CookieCfg) *sessions.Options {
	opts := &sessions.Options{
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	if cfg.IsProd {
		opts.Secure = true
		if cfg.ShareSubdoms && cfg.ParentDomain != "" {
			opts.Domain = "." + cfg.ParentDomain
		}
	} else {
		opts.Secure = false
	}
	return opts
}

// authMiddleware ensures a user is authenticated before accessing protected routes.
// It reads uid/ownerid from the session; on failure it redirects to /login.
func (ctrl *controller) authMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		sw, err := LoadSession(c)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Errorf("cannot load session: %w", err))
		}

		// IMPORTANT: type assertions must match what you store (here: uint).
		var ok bool
		var uid uint
		if v, exists := sw.Values()["uid"]; exists {
			uid, ok = v.(uint)
		}
		if !ok || uid == 0 {
			return c.Redirect(http.StatusSeeOther, "/login")
		}
		c.Set("uid", uid)

		if v, exists := sw.Values()["ownerid"]; exists {
			if ownerid, ok := v.(uint); ok && ownerid != 0 {
				c.Set("ownerid", ownerid)
			} else {
				return c.Redirect(http.StatusSeeOther, "/login")
			}
		} else {
			return c.Redirect(http.StatusSeeOther, "/login")
		}

		// Simple admin flag example.
		if uid == 1 {
			c.Set("is_admin", true)
		}
		return next(c)
	}
}

// login handles GET (render form) and POST (authenticate).
// On successful POST, it stores uid/ownerid and the "persist" flag (remember me) in the session.
// The actual cookie MaxAge is applied automatically by SessionWriter.Save().
func (ctrl *controller) login(c echo.Context) error {
	if c.Request().Method == http.MethodGet {
		m := ctrl.defaultResponseMap(c, "Login")
		return c.Render(http.StatusOK, "login.html", m)
	}

	// POST
	email := strings.TrimSpace(strings.ToLower(c.FormValue("email")))
	password := c.FormValue("password")
	remember := c.FormValue("rememberMe") != ""

	// Authenticate (do not leak whether the user exists).
	user, err := ctrl.model.AuthenticateUser(email, password)
	if err != nil || user == nil {
		if err := AddFlash(c, "error", "Login failed. Please check your input."); err != nil {
			return ErrInvalid(err, "error while saving the session")
		}
		return c.Redirect(http.StatusSeeOther, "/login")
	}

	// Optional: require verified email.
	if user.Verified == false {
		_ = AddFlash(c, "info", "Please confirm your email first. We've sent you instructions if needed.")
		return c.Redirect(http.StatusSeeOther, "/login")
	}

	// Open session and set values. We only set "persist"; Save() will apply cookie options.
	sw, err := LoadSession(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	sw.Values()["uid"] = user.ID
	sw.Values()["ownerid"] = func() uint {
		if user.OwnerID != 0 {
			return user.OwnerID
		}
		return user.ID // fallback for legacy data
	}()
	sw.Values()["persist"] = remember // this controls remember-me behavior

	if err := sw.Save(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	_ = ctrl.model.TouchLastLogin(user) // best-effort
	return c.Redirect(http.StatusSeeOther, "/")
}

// logout clears the session and deletes the cookie.
// We bypass SessionWriter here to force MaxAge = -1 (cookie deletion) regardless of "persist".
func (ctrl *controller) logout(c echo.Context) error {
	sess, err := session.Get("session", c)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}
	delete(sess.Values, "uid")
	delete(sess.Values, "ownerid")
	delete(sess.Values, "csrf")
	delete(sess.Values, "persist")

	// Force-delete the cookie for all browsers (including Safari).
	if sess.Options == nil {
		sess.Options = &sessions.Options{Path: "/"}
	}
	sess.Options.MaxAge = -1

	if err := sess.Save(c.Request(), c.Response()); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}
	_ = AddFlash(c, "success", "You have been signed out.")
	return c.Redirect(http.StatusFound, "/login")
}

// generateRandomToken returns a URL-safe, base64 token and its sha256 hash.
// Use it for verification/signup tokens or password reset tokens.
func generateRandomToken() (token string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", nil, err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(token))
	return token, h[:], nil
}

// constantTimeMatchToken safely compares a provided plaintext token to a stored hash.
func constantTimeMatchToken(providedToken string, storedHash []byte) bool {
	sum := sha256.Sum256([]byte(providedToken))
	return len(storedHash) == len(sum[:]) && hmac.Equal(storedHash, sum[:])
}

// showPasswordResetRequest renders the "request password reset" form (GET).
func (ctrl *controller) showPasswordResetRequest(c echo.Context) error {
	m := ctrl.defaultResponseMap(c, "Password Reset")
	return c.Render(http.StatusOK, "passwordreset.html", m)
}

// handlePasswordResetRequest handles the reset request (POST) in an enumeration-safe way.
func (ctrl *controller) handlePasswordResetRequest(c echo.Context) error {
	logger := c.Get("logger").(*slog.Logger)
	email := strings.TrimSpace(strings.ToLower(c.FormValue("email")))

	genericResponse := func() error {
		_ = AddFlash(c, "info", "If an account exists, we have sent you an email.")
		return c.Redirect(http.StatusSeeOther, "/login")
	}

	user, err := ctrl.model.GetUserByEMail(email)
	if err != nil || user == nil {
		return genericResponse()
	}

	// Generate token + store hash+expiry
	token, tokenHash, err := generateRandomToken()
	if err != nil {
		logger.Error("cannot generate reset token", "error", err)
		return genericResponse()
	}

	user.PasswordResetToken = tokenHash
	user.PasswordResetExpiry = time.Now().UTC().Add(1 * time.Hour)
	if err := ctrl.model.UpdateUser(user); err != nil {
		logger.Error("cannot store reset token", "error", err)
		return genericResponse()
	}

	// Build absolute reset URL like: https://host/password/reset/<token>
	resetURL := fmt.Sprintf("%s://%s/password/reset/%s", c.Scheme(), c.Request().Host, url.PathEscape(token))

	body := fmt.Sprintf(
		"Click the link to reset your password:\n\n%s\n\nThe link is valid for 60 minutes.",
		resetURL,
	)
	_ = ctrl.sendEmail(email, "Reset your password", body)

	return genericResponse()
}

// showPasswordResetForm validates the token and renders the "set new password" form.
// If anything fails (invalid/expired), it redirects with a neutral error message.
func (ctrl *controller) showPasswordResetForm(c echo.Context) error {
	token := c.Param("token")

	sum := sha256.Sum256([]byte(token))
	user, err := ctrl.model.GetUserByResetTokenHashPrefix(sum[:], 16)
	if err != nil || user == nil || user.PasswordResetExpiry.Before(time.Now().UTC()) ||
		!constantTimeMatchToken(token, user.PasswordResetToken) {
		_ = AddFlash(c, "error", "The link is invalid or has expired.")
		return c.Redirect(http.StatusSeeOther, "/login")
	}

	m := ctrl.defaultResponseMap(c, "Set a new password")
	m["token"] = token
	return c.Render(http.StatusOK, "passwordresettoken.html", m)
}

// handlePasswordResetSubmit sets the new password and clears the token.
// Always responds neutrally on failure to avoid leaks.
func (ctrl *controller) handlePasswordResetSubmit(c echo.Context) error {
	token := c.Param("token")
	pass := c.FormValue("newPassword")
	confirm := c.FormValue("confirmPassword")
	logger := c.Get("logger").(*slog.Logger)

	if pass == "" || pass != confirm {
		_ = AddFlash(c, "error", "Please check your input (passwords do not match).")
		return c.Redirect(http.StatusSeeOther, c.Request().RequestURI)
	}

	sum := sha256.Sum256([]byte(token))
	user, err := ctrl.model.GetUserByResetTokenHashPrefix(sum[:], 16)
	if err != nil || user == nil || user.PasswordResetExpiry.Before(time.Now().UTC()) ||
		!constantTimeMatchToken(token, user.PasswordResetToken) {
		_ = AddFlash(c, "error", "The link is invalid or has expired.")
		return c.Redirect(http.StatusSeeOther, "/login")
	}

	if err := ctrl.model.SetPassword(user, pass); err != nil {
		logger.Error("cannot set password", "error", err)
		_ = AddFlash(c, "error", "Internal error. Please try again later.")
		return c.Redirect(http.StatusSeeOther, "/login")
	}
	// Clear token (best-effort after password update).
	user.PasswordResetToken = nil
	user.PasswordResetExpiry = time.Time{}
	if err := ctrl.model.UpdateUser(user); err != nil {
		logger.Error("cannot clear reset token", "error", err)
	}

	_ = AddFlash(c, "success", "Your password has been updated. You can sign in now.")
	return c.Redirect(http.StatusSeeOther, "/login")
}

// register handles GET (render form) and POST (start enumeration-safe signup).
// For POST: if email exists, send sign-in/reset mail; otherwise create a pending signup token.
func (ctrl *controller) register(c echo.Context) error {
	if !ctrl.model.Config.RegistrationAllowed {
		return echo.NewHTTPError(http.StatusForbidden, "Registration is disabled")
	}
	if c.Request().Method == http.MethodGet {
		m := ctrl.defaultResponseMap(c, "Register")
		return c.Render(http.StatusOK, "register.html", m)
	}

	email := strings.TrimSpace(strings.ToLower(c.FormValue("email")))
	password := c.FormValue("password")

	neutral := func() error {
		m := ctrl.defaultResponseMap(c, "Register")
		m["flash_success"] = "If we can create or locate an account for that email, we have sent you an email with next steps."
		return c.Render(http.StatusOK, "register_submitted.html", m)
	}

	existingUser, err := ctrl.model.GetUserByEMail(email)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return neutral()
	}
	if existingUser != nil {
		body := "Someone tried to sign up with your email. If this was you, sign in here or reset your password."
		_ = ctrl.sendEmail(email, "Sign in to billingcat", body)
		return neutral()
	}

	signupToken, tokenHash, err := generateRandomToken()
	if err != nil {
		return neutral()
	}
	if _, err := ctrl.model.CreateSignupToken(email, password, 30*time.Minute, signupToken); err != nil {
		return neutral()
	}

	verifyURL := fmt.Sprintf("%s://%s/verify?token=%s", c.Scheme(), c.Request().Host, url.QueryEscape(signupToken))
	_ = tokenHash

	body := fmt.Sprintf(
		"Please confirm your email for billingcat:\n\n%s\n\nThe link is valid for 30 minutes. If you did not request this, you can ignore this message.",
		verifyURL,
	)
	_ = ctrl.sendEmail(email, "Confirm your email", body)

	return neutral()
}

// verifyEmail consumes the email verification token and opens a short-lived
// gate to /set-password. The short-lived gate is stored in the session; Save()
// applies cookie options automatically.
func (ctrl *controller) verifyEmail(c echo.Context) error {
	token := c.QueryParam("token")
	if token == "" {
		_ = AddFlash(c, "error", "Invalid or expired link.")
		return c.Redirect(http.StatusSeeOther, "/login")
	}

	u, err := ctrl.model.ConsumeSignupToken(token)
	if err != nil || u == nil {
		_ = AddFlash(c, "error", "Invalid or expired link.")
		return c.Redirect(http.StatusSeeOther, "/login")
	}

	// Ensure OwnerID for solo users (idempotent).
	if u.OwnerID == 0 {
		u.OwnerID = u.ID
		_ = ctrl.model.UpdateUser(u) // best-effort
	}

	sw, err := LoadSession(c)
	if err != nil {
		_ = AddFlash(c, "error", "Internal error. Please try again.")
		return c.Redirect(http.StatusSeeOther, "/login")
	}

	// Short-lived gate (~15 minutes) to set password.
	const gateUIDKey = "pw_setup_uid"
	const gateExpKey = "pw_setup_exp" // unix seconds
	sw.Values()[gateUIDKey] = u.ID
	sw.Values()[gateExpKey] = time.Now().Add(15 * time.Minute).Unix()

	if err := sw.Save(); err != nil {
		_ = AddFlash(c, "error", "Internal error. Please try again.")
		return c.Redirect(http.StatusSeeOther, "/login")
	}

	return c.Redirect(http.StatusSeeOther, "/set-password")
}

// showSetPasswordForm renders the password setup page if the short-lived gate is valid.
func (ctrl *controller) showSetPasswordForm(c echo.Context) error {
	sw, err := LoadSession(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	const gateUIDKey = "pw_setup_uid"
	const gateExpKey = "pw_setup_exp"

	uidVal, okUID := sw.Values()[gateUIDKey].(uint)
	expVal, okExp := sw.Values()[gateExpKey].(int64)
	if !okUID || !okExp || time.Now().Unix() > expVal {
		_ = AddFlash(c, "info", "Please start the verification process again.")
		return c.Redirect(http.StatusSeeOther, "/login")
	}

	// Optional: ensure the user still exists.
	if _, err := ctrl.model.GetUserByID(uidVal); err != nil {
		_ = AddFlash(c, "info", "Please start the verification process again.")
		return c.Redirect(http.StatusSeeOther, "/login")
	}

	m := ctrl.defaultResponseMap(c, "Set Password")
	return c.Render(http.StatusOK, "setpassword.html", m)
}

// handleSetPasswordSubmit accepts the new password, saves it, clears the gate,
// and logs the user in with a normal session. If you want remember-me here,
// add a checkbox to the form and set sw.Values()["persist"] accordingly.
func (ctrl *controller) handleSetPasswordSubmit(c echo.Context) error {
	pass := c.FormValue("password")
	confirm := c.FormValue("confirmPassword")

	if pass == "" || pass != confirm {
		_ = AddFlash(c, "error", "Please check your input (passwords do not match).")
		return c.Redirect(http.StatusSeeOther, "/set-password")
	}

	sw, err := LoadSession(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	const gateUIDKey = "pw_setup_uid"
	const gateExpKey = "pw_setup_exp"

	uidVal, okUID := sw.Values()[gateUIDKey].(uint)
	expVal, okExp := sw.Values()[gateExpKey].(int64)
	if !okUID || !okExp || time.Now().Unix() > expVal {
		_ = AddFlash(c, "info", "Your session expired. Please verify again.")
		return c.Redirect(http.StatusSeeOther, "/login")
	}

	// Load user and set password.
	u, err := ctrl.model.GetUserByID(uidVal)
	if err != nil || u == nil {
		_ = AddFlash(c, "error", "Internal error. Please try again.")
		return c.Redirect(http.StatusSeeOther, "/login")
	}

	if err := ctrl.model.SetPassword(u, pass); err != nil {
		_ = AddFlash(c, "error", "Internal error. Please try again.")
		return c.Redirect(http.StatusSeeOther, "/set-password")
	}

	// Ensure the user is marked verified (idempotent).
	if !u.Verified {
		u.Verified = true
	}
	if err := ctrl.model.UpdateUser(u); err != nil {
		_ = AddFlash(c, "error", "Internal error. Please try again.")
		return c.Redirect(http.StatusSeeOther, "/set-password")
	}

	// Clear the gate keys.
	delete(sw.Values(), gateUIDKey)
	delete(sw.Values(), gateExpKey)

	// Establish a normal signed-in session. No remember-me here (unless you add a checkbox).
	sw.Values()["uid"] = u.ID
	sw.Values()["ownerid"] = u.ID
	// NOTE: do not set "persist" here unless your form has a remember-me checkbox.

	if err := sw.Save(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	_ = AddFlash(c, "success", "Your password has been set. Welcome!")
	_ = ctrl.model.TouchLastLogin(u)
	return c.Redirect(http.StatusSeeOther, "/")
}
