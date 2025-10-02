package controller

import (
	"strings"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
)

// SessionWriter kapselt Gorilla-Session-Handling.
// Sie sorgt automatisch dafür, dass persistente "remember me" Optionen
// vor jedem Save angewendet werden.
type SessionWriter struct {
	sess *sessions.Session
	c    echo.Context
}

// LoadSession holt oder erstellt eine Session mit Namen "session".
func LoadSession(c echo.Context) (*SessionWriter, error) {
	sess, err := session.Get("session", c)
	if err != nil {
		return nil, err
	}
	return &SessionWriter{sess: sess, c: c}, nil
}

// Values erlaubt Zugriff auf die Session-Daten (map-artig).
func (sw *SessionWriter) Values() map[interface{}]interface{} {
	return sw.sess.Values
}

// AddFlash hängt eine Flash-Message an.
func (sw *SessionWriter) AddFlash(v interface{}) {
	sw.sess.AddFlash(v)
}

// Save schreibt die Session zurück – inklusive richtiger CookieOptions.
func (sw *SessionWriter) Save() error {
	applySessionOptionsFromPersist(sw.c, sw.sess)
	return sw.sess.Save(sw.c.Request(), sw.c.Response())
}

// Helper: setzt Session-Options basierend auf persist-Flag.
func applySessionOptionsFromPersist(c echo.Context, sess *sessions.Session) {
	persist, _ := sess.Values["persist"].(bool)
	maxAge := 0
	if persist {
		maxAge = 60 * 60 * 24 * 365 // 1 Jahr
	}

	// Besser: aus deiner globalen Config ziehen; hier heuristisch
	isHTTPS := c.Scheme() == "https" || c.Request().TLS != nil ||
		strings.EqualFold(c.Request().Header.Get("X-Forwarded-Proto"), "https")

	cfg := CookieCfg{
		IsProd:       isHTTPS,
		ShareSubdoms: false,
		ParentDomain: "billingcat.de",
	}
	sess.Options = cookieOptions(maxAge, cfg)
}
