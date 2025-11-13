package controller

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/billingcat/crm/model"
	"github.com/labstack/echo/v4"
)

// adminInit wires the /admin routes.
func (ctrl *controller) adminInit(e *echo.Echo) {
	g := e.Group("/admin", ctrl.authMiddleware, ctrl.adminMiddleware)

	// Users list with optional search & pagination.
	g.GET("/users", ctrl.adminUsersList)
	// Show list + form
	g.GET("/invitations", ctrl.adminInvitationsPage)

	// Handle form submission to create a new invitation
	g.POST("/invitations", ctrl.adminCreateInvitation)
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

func (ctrl *controller) adminInvitationsPage(c echo.Context) error {
	ctx := c.Request().Context()
	m := ctrl.defaultResponseMap(c, "Invitations")

	invitations, err := ctrl.model.ListInvitations(ctx)
	if err != nil {
		return err
	}
	m["URLprefix"] = fmt.Sprintf("%s://%s", c.Scheme(), c.Request().Host)
	m["invitations"] = invitations

	return c.Render(http.StatusOK, "admin_invitations.html", m)
}

// adminCreateInvitation handles POSTs from the invitation form and
// creates a new invitation record.
func (ctrl *controller) adminCreateInvitation(c echo.Context) error {
	ctx := c.Request().Context()

	email := strings.TrimSpace(c.FormValue("email"))
	expiresStr := strings.TrimSpace(c.FormValue("expires_at"))

	token, err := GenerateToken(32)
	if err != nil {
		return err
	}

	inv := &model.Invitation{
		Token:     token,
		Email:     email,
		CreatedAt: time.Now(),
	}

	// Parse optional expiration date coming from <input type="date">
	if expiresStr != "" {
		// For <input type="date"> the browser sends YYYY-MM-DD
		t, err := time.ParseInLocation("2006-01-02", expiresStr, time.Local)
		if err != nil {
			// Debug: log invalid date instead of silently ignoring it
			c.Logger().Warnf("invalid expires_at value %q: %v", expiresStr, err)
		} else {
			inv.ExpiresAt = &t
		}
	}

	// Debug: check what we are about to write
	c.Logger().Debugf("creating invitation: token=%s email=%s expires_at=%v",
		inv.Token, inv.Email, inv.ExpiresAt)

	if err := ctrl.model.CreateInvitation(ctx, inv); err != nil {
		return err
	}
	return c.Redirect(http.StatusSeeOther, "/admin_invitation_created.html")
}

func GenerateToken(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil // z.B. 32 Bytes -> 64 Hex-Zeichen
}
