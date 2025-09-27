package controller

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

func (ctrl *controller) adminInit(e *echo.Echo) {
	if ctrl.model.Config.Mode == "development" {
		e.GET("/__admin/create-token", ctrl.adminCreateToken)
	}
}

// never use in production!
func (ctrl *controller) adminCreateToken(c echo.Context) error {
	ownerID := uint(1)
	userID := uint(1)

	// Token erstellen
	plain, rec, err := ctrl.model.CreateAPIToken(ownerID, &userID, "AdminBootstrap", "invoices:read", nil)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	// Einmalig Klartext-Token zurückgeben
	return c.JSON(http.StatusOK, map[string]any{
		"id":      rec.ID,
		"prefix":  rec.TokenPrefix,
		"token":   plain,
		"ownerID": rec.OwnerID,
		"scope":   rec.Scope,
		"created": rec.CreatedAt.Format(time.RFC3339),
	})
}
