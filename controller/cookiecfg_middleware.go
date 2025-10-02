package controller

import (
	"github.com/labstack/echo/v4"
)

// CookieCfgMiddleware injects a CookieCfg into the Echo context for each request.
// It uses your global app config (ctrl.model.Config) to decide prod/dev and
// whether cookies should be shared across subdomains.
func (ctrl *controller) CookieCfgMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		cfg := CookieCfg{
			IsProd:       ctrl.model.Config.Mode == "production",
			ShareSubdoms: false,           // set true if you need cross-subdomain cookies
			ParentDomain: "billingcat.de", // only relevant if ShareSubdoms=true
		}
		c.Set("cookiecfg", cfg)
		return next(c)
	}
}
