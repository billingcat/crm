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
type CookieCfg struct {
	IsProd       bool   // true in production
	ShareSubdoms bool   // true to share cookie across subdomains
	ParentDomain string // e.g. "billingcat.de" (only if ShareSubdoms = true)
}

// cookieOptions builds secure cookie options based on environment.
// - In production, Secure MUST be true (HTTPS).
// - Domain is only set if you truly need cross-subdomain sessions.
// - SameSite=Lax is safe-by-default for typical app flows.
func cookieOptions(maxAge int, cfg CookieCfg) *sessions.Options {
	opts := &sessions.Options{
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	if cfg.IsProd {
		opts.Secure = true // e.g., https://app.billingcat.de MUST be true
		if cfg.ShareSubdoms && cfg.ParentDomain != "" {
			opts.Domain = "." + cfg.ParentDomain // e.g., ".billingcat.de"
		}
	} else {
		// Local development on http://localhost
		opts.Secure = false // otherwise cookie won't be set on http
		// Domain left empty (host-only for "localhost")
	}
	return opts
}

// authMiddleware ensures a user is authenticated before accessing protected routes.
// It populates "uid" and "ownerid" into the context on success, or redirects to /login.
func (ctrl *controller) authMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		sess, err := session.Get("session", c)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Errorf("cannot get session %w", err))
		}
		if uid, ok := sess.Values["uid"].(uint); ok {
			c.Set("uid", uid)
		} else {
			return c.Redirect(http.StatusSeeOther, "/login")
		}
		if ownerid, ok := sess.Values["ownerid"].(uint); ok {
			c.Set("ownerid", ownerid)
		} else {
			return c.Redirect(http.StatusSeeOther, "/login")
		}
		return next(c)
	}
}

// login handles GET (render form) and POST (authenticate) for password-based login.
// For failed attempts, it returns a neutral message (no user enumeration).
func (ctrl *controller) login(c echo.Context) error {
	if c.Request().Method == http.MethodGet {
		m := ctrl.defaultResponseMap(c, "Login")
		return c.Render(http.StatusOK, "login.html", m)
	}

	// POST
	email := strings.TrimSpace(strings.ToLower(c.FormValue("email")))
	password := c.FormValue("password")
	remember := c.FormValue("rememberMe") != ""

	// Authenticate using model; returns ErrInvalidPassword or gorm.ErrRecordNotFound under the hood.
	user, err := ctrl.model.AuthenticateUser(email, password)
	if err != nil || user == nil {
		// Deliberately neutral – do not leak whether email exists.
		if err := AddFlash(c, "error", "Login failed. Please check your input."); err != nil {
			return ErrInvalid(err, "error while saving the session")
		}
		return c.Redirect(http.StatusSeeOther, "/login")
	}

	// Optional: require verified email (if you added Verified to User)
	// If not verified, act neutral and nudge to check inbox.
	if user.Verified == false {
		_ = AddFlash(c, "info", "Please confirm your email first. We've sent you instructions if needed.")
		return c.Redirect(http.StatusSeeOther, "/login")
	}

	sess, err := session.Get("session", c)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	// "Remember me" = 1 year; otherwise, session cookie (MaxAge=0)
	maxAge := 0
	if remember {
		maxAge = 60 * 60 * 24 * 365 // 1 year
	}

	// Secure cookie options depending on environment
	opts := cookieOptions(maxAge, CookieCfg{
		IsProd:       ctrl.model.Config.Mode == "production",
		ShareSubdoms: false,           // set to true only if you really need it
		ParentDomain: "billingcat.de", // only relevant if ShareSubdoms=true
	})
	// NOTE: If you rely on cross-site redirects during OAuth and need the cookie sent cross-site,
	// you would use SameSite=None + Secure=true (production only).
	// opts.SameSite = http.SameSiteNoneMode // ONLY with opts.Secure=true in production

	sess.Options = opts

	// Today, uid == ownerid. If you add teams later, store OwnerID explicitly.
	sess.Values["uid"] = user.ID
	sess.Values["ownerid"] = func() uint {
		if user.OwnerID != 0 {
			return user.OwnerID
		}
		return user.ID // fallback für Altbestände
	}()

	if err := sess.Save(c.Request(), c.Response()); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}
	_ = ctrl.model.TouchLastLogin(user) // non-blocking ok
	return c.Redirect(http.StatusSeeOther, "/")
}

// logout destroys the session and redirects to /login.
// Sets MaxAge=-1 to ensure browsers like Safari actually delete it.
func (ctrl *controller) logout(c echo.Context) error {
	sess, err := session.Get("session", c)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}
	delete(sess.Values, "uid")
	delete(sess.Values, "ownerid")
	delete(sess.Values, "csrf")

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

// constantTimeMatchToken compares a provided plaintext token to a stored hash safely.
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
// It always returns the same response, whether a user exists or not.
func (ctrl *controller) handlePasswordResetRequest(c echo.Context) error {
	logger := c.Get("logger").(*slog.Logger)
	email := strings.TrimSpace(strings.ToLower(c.FormValue("email")))

	genericResponse := func() error {
		_ = AddFlash(c, "info", "If an account exists, we have sent you an email.")
		return c.Redirect(http.StatusSeeOther, "/login")
	}

	user, err := ctrl.model.GetUserByEMail(email)
	if err != nil || user == nil {
		// intentionally identical outward response
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
	// Clear token
	user.PasswordResetToken = nil
	user.PasswordResetExpiry = time.Time{}
	if err := ctrl.model.UpdateUser(user); err != nil {
		logger.Error("cannot clear reset token", "error", err)
		// Password is already set; continue with success.
	}
	_ = AddFlash(c, "success", "Your password has been updated. You can sign in now.")
	return c.Redirect(http.StatusSeeOther, "/login")
}

// register handles GET (render form) and POST (start enumeration-safe signup).
// On POST:
//   - If the email already exists, send a sign-in/reset email.
//   - If it’s new, create a pending signup token and send a verification email.
//   - In both cases, respond with the same neutral success message.
func (ctrl *controller) register(c echo.Context) error {
	if !ctrl.model.Config.RegistrationAllowed {
		return echo.NewHTTPError(http.StatusForbidden, "Registration is disabled")
	}
	if c.Request().Method == http.MethodGet {
		m := ctrl.defaultResponseMap(c, "Register")
		return c.Render(http.StatusOK, "register.html", m)
	}

	email := strings.TrimSpace(strings.ToLower(c.FormValue("email")))
	password := c.FormValue("password") // optional: server-side password policy checks

	// Unified outward response (avoid account enumeration)
	neutral := func() error {
		m := ctrl.defaultResponseMap(c, "Register")
		m["flash_success"] = "If we can create or locate an account for that email, we have sent you an email with next steps."
		return c.Render(http.StatusOK, "register_submitted.html", m)
	}

	// Look up existing user
	existingUser, err := ctrl.model.GetUserByEMail(email)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		// Log internally, present neutral response to user
		return neutral()
	}
	if existingUser != nil {
		// Existing account → send sign-in or reset email (do not reveal state)
		body := "Someone tried to sign up with your email. If this was you, sign in here or reset your password."
		_ = ctrl.sendEmail(email, "Sign in to billingcat", body) // replace with magic-link/reset in your project
		return neutral()
	}

	// New email → create signup token and send verification link
	signupToken, tokenHash, err := generateRandomToken()
	if err != nil {
		return neutral()
	}
	if _, err := ctrl.model.CreateSignupToken(email, password, 30*time.Minute, signupToken); err != nil {
		// Handle potential races/uniqueness quietly; still neutral outside
		return neutral()
	}

	// Build verify URL like: https://host/verify?token=...
	verifyURL := fmt.Sprintf("%s://%s/verify?token=%s", c.Scheme(), c.Request().Host, url.QueryEscape(signupToken))
	_ = tokenHash // tokenHash is stored by CreateSignupToken via sha256; kept here only for clarity.

	body := fmt.Sprintf(
		"Please confirm your email for billingcat:\n\n%s\n\nThe link is valid for 30 minutes. If you did not request this, you can ignore this message.",
		verifyURL,
	)
	_ = ctrl.sendEmail(email, "Confirm your email", body)

	return neutral()
}

// verifyEmail consumes the email verification token (verify-first).
// On success it creates/verifies the user (via model.ConsumeSignupToken)
// and opens a short-lived session gate to /set-password.
func (ctrl *controller) verifyEmail(c echo.Context) error {
	token := c.QueryParam("token")
	if token == "" {
		_ = AddFlash(c, "error", "Invalid or expired link.")
		return c.Redirect(http.StatusSeeOther, "/login")
	}

	u, err := ctrl.model.ConsumeSignupToken(token)
	if err != nil || u == nil {
		// Deliberately neutral outwardly.
		_ = AddFlash(c, "error", "Invalid or expired link.")
		return c.Redirect(http.StatusSeeOther, "/login")
	}

	// ensure OwnerID is set for solo users
	if u.OwnerID == 0 {
		u.OwnerID = u.ID
		_ = ctrl.model.UpdateUser(u) // best-effort
	}
	// Open a short-lived gate (e.g., 15 minutes) to let the user set a password.
	sess, err := session.Get("session", c)
	if err != nil {
		_ = AddFlash(c, "error", "Internal error. Please try again.")
		return c.Redirect(http.StatusSeeOther, "/login")
	}

	// We keep the existing cookie options; the session is already configured elsewhere.
	// Gate keys – keep them short and explicit.
	const gateUIDKey = "pw_setup_uid"
	const gateExpKey = "pw_setup_exp" // unix seconds

	sess.Values[gateUIDKey] = u.ID
	sess.Values[gateExpKey] = time.Now().Add(15 * time.Minute).Unix()

	if err := sess.Save(c.Request(), c.Response()); err != nil {
		_ = AddFlash(c, "error", "Internal error. Please try again.")
		return c.Redirect(http.StatusSeeOther, "/login")
	}

	// Redirect to the password setup form
	return c.Redirect(http.StatusSeeOther, "/set-password")
}

// showSetPasswordForm renders the password setup page if the short-lived gate is valid.
// Otherwise it redirects to /login with a neutral message.
func (ctrl *controller) showSetPasswordForm(c echo.Context) error {
	sess, err := session.Get("session", c)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	uidVal, okUID := sess.Values["pw_setup_uid"].(uint)
	expVal, okExp := sess.Values["pw_setup_exp"].(int64)
	if !okUID || !okExp || time.Now().Unix() > expVal {
		_ = AddFlash(c, "info", "Please start the verification process again.")
		return c.Redirect(http.StatusSeeOther, "/login")
	}

	// Optional: ensure the user still exists
	if _, err := ctrl.model.GetUserByID(uidVal); err != nil {
		_ = AddFlash(c, "info", "Please start the verification process again.")
		return c.Redirect(http.StatusSeeOther, "/login")
	}

	m := ctrl.defaultResponseMap(c, "Set Password")
	return c.Render(http.StatusOK, "setpassword.html", m)
}

// handleSetPasswordSubmit accepts the new password, saves it, clears the gate,
// and logs the user in with a normal session.
func (ctrl *controller) handleSetPasswordSubmit(c echo.Context) error {
	pass := c.FormValue("password")
	confirm := c.FormValue("confirmPassword")

	if pass == "" || pass != confirm {
		_ = AddFlash(c, "error", "Please check your input (passwords do not match).")
		return c.Redirect(http.StatusSeeOther, "/set-password")
	}

	sess, err := session.Get("session", c)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	uidVal, okUID := sess.Values["pw_setup_uid"].(uint)
	expVal, okExp := sess.Values["pw_setup_exp"].(int64)
	if !okUID || !okExp || time.Now().Unix() > expVal {
		_ = AddFlash(c, "info", "Your session expired. Please verify again.")
		return c.Redirect(http.StatusSeeOther, "/login")
	}

	// Load user and set password
	u, err := ctrl.model.GetUserByID(uidVal)
	if err != nil || u == nil {
		_ = AddFlash(c, "error", "Internal error. Please try again.")
		return c.Redirect(http.StatusSeeOther, "/login")
	}

	if err := ctrl.model.SetPassword(u, pass); err != nil {
		_ = AddFlash(c, "error", "Internal error. Please try again.")
		return c.Redirect(http.StatusSeeOther, "/set-password")
	}

	// Ensure the user is marked verified (idempotent)
	if !u.Verified {
		u.Verified = true
	}
	if err := ctrl.model.UpdateUser(u); err != nil {
		_ = AddFlash(c, "error", "Internal error. Please try again.")
		return c.Redirect(http.StatusSeeOther, "/set-password")
	}

	// Clear the gate keys
	delete(sess.Values, "pw_setup_uid")
	delete(sess.Values, "pw_setup_exp")

	// Establish a normal signed-in session (no "remember me" here).
	// If you want "remember me" from here, add a checkbox to the form and set MaxAge accordingly.
	opts := cookieOptions(0, CookieCfg{
		IsProd:       ctrl.model.Config.Mode == "production",
		ShareSubdoms: false,
		ParentDomain: "billingcat.de",
	})
	sess.Options = opts
	sess.Values["uid"] = u.ID
	sess.Values["ownerid"] = u.ID

	if err := sess.Save(c.Request(), c.Response()); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	_ = AddFlash(c, "success", "Your password has been set. Welcome!")
	_ = ctrl.model.TouchLastLogin(u)
	return c.Redirect(http.StatusSeeOther, "/")
}
