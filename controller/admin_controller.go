package controller

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

// adminInit wires the /admin routes.
func (ctrl *controller) adminInit(e *echo.Echo) {
	g := e.Group("/admin", ctrl.authMiddleware, ctrl.adminMiddleware)

	// Users list with optional search & pagination.
	g.GET("/users", ctrl.adminUsersList)
}

// adminMiddleware ensures only privileged users can access /admin.
func (ctrl *controller) adminMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if b, ok := c.Get("is_admin").(bool); ok && b {
			return next(c)
		}
		return echo.NewHTTPError(http.StatusForbidden, "Not found")
	}
}

// adminUsersList renders a simple searchable, paginated list of users.
func (ctrl *controller) adminUsersList(c echo.Context) error {
	m := ctrl.defaultResponseMap(c, "Benutzer (Admin)")
	q := strings.TrimSpace(c.QueryParam("q"))

	// Pagination params
	const defaultPerPage = 20
	const maxPerPage = 100

	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(c.QueryParam("per"))
	if perPage <= 0 || perPage > maxPerPage {
		perPage = defaultPerPage
	}
	offset := (page - 1) * perPage

	users, total, err := ctrl.model.ListUsers(q, offset, perPage)
	if err != nil {
		return err
	}

	// Build simple pagination info
	totalPages := int((total + int64(perPage) - 1) / int64(perPage))
	hasPrev := page > 1
	hasNext := page < totalPages

	// Data for the view
	m["q"] = q
	m["users"] = users
	m["page"] = page
	m["per"] = perPage
	m["total"] = total
	m["totalPages"] = totalPages
	m["hasPrev"] = hasPrev
	m["hasNext"] = hasNext

	// Helper URLs for buttons/links
	buildURL := func(p int) string {
		// Keep q and per in the query while changing page
		return "/admin/users?q=" + url.QueryEscape(q) +
			"&per=" + strconv.Itoa(perPage) +
			"&page=" + strconv.Itoa(p)
	}

	m["prevURL"] = ""
	m["nextURL"] = ""
	if hasPrev {
		m["prevURL"] = buildURL(page - 1)
	}
	if hasNext {
		m["nextURL"] = buildURL(page + 1)
	}
	m["selfURL"] = buildURL(page)

	return c.Render(http.StatusOK, "admin_users.html", m)
}
