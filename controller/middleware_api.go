// controller/api_auth.go
package controller

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

type ctxKey string

const (
	ctxOwnerID ctxKey = "api_owner_id"
	ctxUserID  ctxKey = "api_user_id"
	ctxScopes  ctxKey = "api_scopes"
)

func (ctrl *controller) APIKeyAuthMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			auth := c.Request().Header.Get("Authorization")
			if auth == "" {
				return c.JSON(http.StatusUnauthorized, apiError("missing_token", "Provide Authorization header"))
			}
			parts := strings.SplitN(auth, " ", 2)
			if len(parts) != 2 || (!strings.EqualFold(parts[0], "Bearer") && !strings.EqualFold(parts[0], "Api-Key")) {
				return c.JSON(http.StatusUnauthorized, apiError("bad_token", "Use Bearer or Api-Key"))
			}
			rec, err := ctrl.model.ValidateAPIToken(parts[1])
			if err != nil {
				return c.JSON(http.StatusUnauthorized, apiError("unauthorized", "Unauthorized"))
			}

			c.Set(string(ctxOwnerID), rec.OwnerID)
			c.Set(string(ctxUserID), rec.UserID) // kann nil sein
			c.Set(string(ctxScopes), rec.Scope)
			return next(c)
		}
	}
}

// kleine Getter
func apiOwnerID(c echo.Context) uint {
	if v, ok := c.Get(string(ctxOwnerID)).(uint); ok {
		return v
	}
	return 0
}
func apiScopes(c echo.Context) string {
	if v, ok := c.Get(string(ctxScopes)).(string); ok {
		return v
	}
	return ""
}
