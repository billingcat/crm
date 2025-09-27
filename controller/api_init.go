// controller/api_init.go
package controller

import "github.com/labstack/echo/v4"

func (ctrl *controller) apiInit(e *echo.Echo) {
	api := e.Group("/api/v1")
	api.Use(ctrl.APIKeyAuthMiddleware())

	// Token-Management
	api.POST("/tokens", ctrl.apiCreateToken)
	api.DELETE("/tokens/:id", ctrl.apiRevokeToken)

	// hier deine bereits vorhandenen API-Routen, z.B.:
	api.GET("/invoices", ctrl.apiInvoiceList)
	api.GET("/invoices/:id", ctrl.apiInvoiceGet)
}
