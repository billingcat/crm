// controller/api_init.go
package controller

import "github.com/labstack/echo/v4"

func (ctrl *controller) apiInit(e *echo.Echo) {
	api := e.Group("/api/v1")
	api.Use(ctrl.APIKeyAuthMiddleware())

	// Token-Management
	api.POST("/tokens", ctrl.apiCreateToken)
	api.DELETE("/tokens/:id", ctrl.apiRevokeToken)

	// Invoices
	api.GET("/invoices", ctrl.apiInvoiceList)
	api.GET("/invoices/:id", ctrl.apiInvoiceGet)

	// Customers
	api.GET("/customers", ctrl.apiCustomerList)
	api.GET("/customers/:id", ctrl.apiCustomerGet)
	api.POST("/customers", ctrl.apiCustomerCreate)
}
