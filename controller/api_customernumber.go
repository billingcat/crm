package controller

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

func (ctrl *controller) customernumberInit(e *echo.Echo) {
	g := e.Group("/api/customer-number")
	g.Use(ctrl.authMiddleware)
	g.GET("/check", ctrl.apiCheckCustomerNumber)
}

// apiCheckCustomerNumber delegates to the model-layer and never touches the DB directly here.
func (ctrl *controller) apiCheckCustomerNumber(c echo.Context) error {
	num := c.QueryParam("num")
	excludeStr := c.QueryParam("exclude")

	var excludeID uint
	if excludeStr != "" {
		if v, err := strconv.ParseUint(excludeStr, 10, 64); err == nil {
			excludeID = uint(v)
		}
	}
	ok, msg, err := ctrl.model.CheckCustomerNumber(c.Request().Context(), num, excludeID)
	if err != nil {
		// Keep a generic message for the client; log server-side details elsewhere if needed.
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"ok":      false,
			"message": msg,
		})
	}
	// msg may be empty on ok=true; the frontend already handles state messaging.
	return c.JSON(http.StatusOK, echo.Map{
		"ok":      ok,
		"message": msg,
	})
}
