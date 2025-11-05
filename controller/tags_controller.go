package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

func (ctrl *controller) tagsInit(e *echo.Echo) {
	g := e.Group("/tags")
	g.Use(ctrl.authMiddleware)
	g.GET("", ctrl.tagsSuggest)
}

func (ctrl *controller) tagsSuggest(c echo.Context) error {
	ownerID := c.Get("ownerid").(uint)
	q := strings.TrimSpace(c.QueryParam("q"))
	limit := 10
	if s := strings.TrimSpace(c.QueryParam("limit")); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			limit = n
		}
	}

	names, err := ctrl.model.SuggestTagNames(ownerID, q, limit)
	if err != nil {
		return ErrInvalid(err, "failed to query tags")
	}
	return c.JSON(http.StatusOK, names)
}
