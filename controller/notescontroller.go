package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/billingcat/crm/model"

	"github.com/labstack/echo/v4"
)

func (ctrl *controller) noteInit(e *echo.Echo) {
	g := e.Group("/notes")
	g.Use(ctrl.authMiddleware)
	g.POST("/create", ctrl.CreateNote)
	g.POST("/update/:id", ctrl.UpdateNote)
}

func (ctrl *controller) CreateNote(c echo.Context) error {
	var n model.Note
	if err := c.Bind(&n); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Ungültige Eingaben")
	}

	// Set mandatory fields
	ownerID := c.Get("ownerid").(uint)
	userid := c.Get("uid").(uint)
	n.OwnerID = ownerID
	n.AuthorID = userid
	n.EditedAt = time.Now()

	if err := ctrl.model.CreateNote(&n); err != nil {
		return ErrInvalid(err, "Note konnte nicht gespeichert werden")
	}

	// Redirect back to parent entity
	if n.ParentType == model.ParentTypeCompany {
		return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/company/%d", n.ParentID))
	}
	if n.ParentType == model.ParentTypePerson {
		return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/person/%d", n.ParentID))
	}
	return c.NoContent(http.StatusOK)
}

func (ctrl *controller) UpdateNote(c echo.Context) error {
	authorID := c.Get("uid").(uint)
	ownerID := c.Get("ownerid").(uint)

	nid64, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	noteID := uint(nid64)

	var form struct {
		Title string `form:"title"`
		Body  string `form:"body"`
		Tags  string `form:"tags"`
	}
	if err := c.Bind(&form); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Ungültige Eingaben")
	}

	n, err := ctrl.model.UpdateNoteContentAsAuthor(ownerID, authorID, noteID, form.Title, form.Body, form.Tags)
	if err != nil {
		if strings.Contains(err.Error(), "forbidden") {
			return echo.NewHTTPError(http.StatusForbidden, "Keine Berechtigung")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Notiz konnte nicht aktualisiert werden")
	}

	switch n.ParentType {
	case model.ParentTypeCompany:
		return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/company/%d", n.ParentID))
	case model.ParentTypePerson:
		return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/person/%d", n.ParentID))
	default:
		return c.NoContent(http.StatusOK)
	}
}
