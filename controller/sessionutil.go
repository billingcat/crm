package controller

import (
	"errors"
	"log"
	"strings"

	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
)

// SessionWriter is a thin wrapper around gorilla/sessions that ensures
// cookie options (MaxAge, Secure, Domain, SameSite) are applied consistently
// before saving. This avoids accidentally overwriting a persistent "remember me"
// cookie with a temporary one when saving flash messages or other values.
type SessionWriter struct {
	sess *sessions.Session
	c    echo.Context
}

// LoadSession retrieves the session named "session" from the Echo context.
// It returns a SessionWriter that you should use to read/write values and Save().
func LoadSession(c echo.Context) (*SessionWriter, error) {
	sess, err := session.Get("session", c)
	if err != nil {
		if isRecoverableSessionError(err) {
			// Treat invalid cookie as "no session" and start fresh.
			// Session object is still usable and will overwrite the cookie
			// on Save().
			log.Printf("invalid session cookie, starting fresh: %v", err)
		} else {
			return nil, err
		}
	}
	return &SessionWriter{sess: sess, c: c}, nil
}

// Values gives access to the session data map. Use it to set or read keys:
//
//	sw.Values()["uid"] = user.ID
func (sw *SessionWriter) Values() map[any]any {
	return sw.sess.Values
}

// AddFlash appends a flash message to the session. It does not save automatically;
// call sw.Save() afterwards.
func (sw *SessionWriter) AddFlash(v any) {
	sw.sess.AddFlash(v)
}

// Save persists the session back to the client. It automatically reapplies
// cookie options based on the "persist" flag stored in the session.
func (sw *SessionWriter) Save() error {
	applySessionOptionsFromPersist(sw.c, sw.sess)
	return sw.sess.Save(sw.c.Request(), sw.c.Response())
}

// applySessionOptionsFromPersist adjusts the session.Options before saving.
// It checks for a boolean flag "persist" in the session values:
//   - If true, MaxAge is set to ~1 year (remember me).
//   - If false, MaxAge=0 (session cookie).
//
// Secure/Domain/SameSite are set according to environment.
func applySessionOptionsFromPersist(c echo.Context, sess *sessions.Session) {
	persist, _ := sess.Values["persist"].(bool)
	maxAge := 0
	if persist {
		maxAge = 60 * 60 * 24 * 365 // 1 year
	}

	// Prefer the CookieCfg from context (set by CookieCfgMiddleware).
	cfgAny := c.Get("cookiecfg")
	cfg, ok := cfgAny.(CookieCfg)
	if !ok {
		// Fallback to safe defaults if middleware not applied.
		cfg = CookieCfg{
			IsProd:       false,
			ShareSubdoms: false,
			ParentDomain: "",
		}
	}

	sess.Options = cookieOptions(maxAge, cfg)
}

// GetSessionValue returns a string value from the session, or "" if not found.
func GetSessionValue(c echo.Context, key string) string {
	sw, err := LoadSession(c)
	if err != nil {
		return ""
	}
	val, _ := sw.Values()[key].(string)
	return val
}

// SetSessionValue sets a key/value in the session and saves immediately.
func SetSessionValue(c echo.Context, key, value string) error {
	sw, err := LoadSession(c)
	if err != nil {
		return err
	}
	sw.Values()[key] = value
	return sw.Save()
}

// DeleteSessionValue removes a key from the session and saves immediately.
func DeleteSessionValue(c echo.Context, key string) error {
	sw, err := LoadSession(c)
	if err != nil {
		return err
	}
	delete(sw.Values(), key)
	return sw.Save()
}

// isRecoverableSessionError checks whether the given error from session.Get()
// indicates an invalid or old session cookie that can be treated as "no session".
func isRecoverableSessionError(err error) bool {
	if err == nil {
		return false
	}

	// Typical securecookie decode error
	if strings.Contains(err.Error(), "securecookie: the value is not valid") {
		return true
	}

	var scErr securecookie.Error
	if errors.As(err, &scErr) {
		return true
	}

	return false
}

// ClearSession invalidates the current session cookie and clears all values.
// It treats invalid/old cookies as "nothing to clear".
func ClearSession(c echo.Context) error {
	sess, err := session.Get("session", c)
	if err != nil && !isRecoverableSessionError(err) {
		return err
	}
	if sess == nil {
		sess = sessions.NewSession(nil, "session")
	}

	for k := range sess.Values {
		delete(sess.Values, k)
	}

	sess.Options.MaxAge = -1

	return sess.Save(c.Request(), c.Response())
}
