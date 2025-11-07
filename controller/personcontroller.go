package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/billingcat/crm/model"

	"github.com/go-playground/form/v4"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// personInit registers routes for creating, viewing, editing, deleting,
// and tagging people (contacts). All endpoints are authenticated.
func (ctrl *controller) personInit(e *echo.Echo) {
	g := e.Group("/person")
	g.Use(ctrl.authMiddleware)
	g.GET("/new", ctrl.personnew)
	g.GET("/new/:company", ctrl.personnew)
	g.POST("/new", ctrl.personnew)
	g.GET("/:id/:name", ctrl.persondetail)
	g.GET("/:id", ctrl.persondetail)
	g.GET("/edit/:id", ctrl.personedit)
	g.POST("/edit/:id", ctrl.personedit)
	g.DELETE("/delete/:id", ctrl.deletePersonWithID)
	g.POST("/:id/tags", ctrl.personTagsUpdate)
}

// personForm models the HTML form payload for creating/updating a person.
// Note: "Tags" may be submitted multiple times (e.g., multiple inputs name="tags").
type personForm struct {
	Name     string            `form:"name"`
	Firma    int               `form:"firma"` // company id (kept German to match form name)
	Email    string            `form:"email"`
	Funktion string            `form:"funktion"` // job title/role (kept German to match form name)
	Phone    []contactInfoForm `form:"phone"`    // phone[i].type, phone[i].label, phone[i].value
	Tags     []string          `form:"tags"`     // multiple inputs
}

// personnew serves both GET (render form) and POST (create person).
// GET /person/new           → blank form
// GET /person/new/:company  → pre-filter companies list to the given company
// POST /person/new          → create person and redirect to its detail page
func (ctrl *controller) personnew(c echo.Context) error {
	m := ctrl.defaultResponseMap(c, "Create New Contact")
	ownerID := c.Get("ownerid").(uint)

	switch c.Request().Method {
	case http.MethodGet:
		// If a company id is provided, load just that company; otherwise list all companies.
		companyID := c.Param("company")
		if companyID != "" {
			cmpy, err := ctrl.model.LoadCompany(companyID, ownerID)
			if err != nil {
				return ErrInvalid(err, "Error loading company")
			}
			m["companies"] = []*model.Company{cmpy}
		} else {
			var err error
			m["companies"], err = ctrl.model.LoadAllCompanies(ownerID)
			if err != nil {
				return ErrInvalid(err, "Error loading companies")
			}
		}
		m["persondetail"] = &model.Person{}
		m["action"] = "/person/new"
		m["submit"] = "Create Contact"
		// Cancel goes back to home page unless a company is preselected
		if companyID != "" {
			m["cancel"] = fmt.Sprintf("/company/%s", companyID)
		} else {
			m["cancel"] = "/"
		}
		m["showremove"] = false
		return c.Render(http.StatusOK, "personedit.html", m)

	case http.MethodPost:
		// Parse form (supports arrays like phone[i].*)
		if err := c.Request().ParseForm(); err != nil {
			return ErrInvalid(err, "Error parsing form data")
		}
		var pf personForm
		dec := form.NewDecoder()
		if err := dec.Decode(&pf, c.Request().Form); err != nil {
			return ErrInvalid(err, "Error decoding form data")
		}

		personDB := model.Person{
			Name:      strings.TrimSpace(pf.Name),
			EMail:     strings.TrimSpace(pf.Email),
			Position:  strings.TrimSpace(pf.Funktion),
			CompanyID: pf.Firma,
			OwnerID:   ownerID,
		}

		// Collect ContactInfos (skip empties; default type=phone when missing)
		for _, ci := range pf.Phone {
			ci.Type = strings.TrimSpace(ci.Type)
			ci.Label = strings.TrimSpace(ci.Label)
			ci.Value = strings.TrimSpace(ci.Value)
			if ci.Value == "" {
				continue
			}
			if ci.Type == "" {
				ci.Type = "phone"
			}
			personDB.ContactInfos = append(personDB.ContactInfos, model.ContactInfo{
				Type:       ci.Type,
				Label:      ci.Label,
				Value:      ci.Value,
				OwnerID:    ownerID,
				ParentType: model.ParentTypePerson, // safer than a magic string
				// ParentID is set by SavePerson after create
			})
		}

		// Collect tags from multiple inputs name="tags"
		tagNames := normalizeSliceInput(pf.Tags)

		// SavePerson upserts the person, replaces ContactInfos, and applies tag semantics transactionally.
		if err := ctrl.model.SavePerson(&personDB, ownerID, tagNames); err != nil {
			return ErrInvalid(err, "Error creating contact")
		}
		return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/person/%d", personDB.ID))
	}
	return nil
}

// deletePersonWithID deletes a person by id after verifying ownership.
func (ctrl *controller) deletePersonWithID(c echo.Context) error {
	ownerID := c.Get("ownerid").(uint)
	personID := c.Param("id")
	personDB, err := ctrl.model.LoadPerson(personID, ownerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrInvalid(err, "Contact not found")
		}
	}
	if personDB.OwnerID != ownerID {
		return echo.ErrForbidden
	}
	if err := ctrl.model.RemovePerson(personID, ownerID); err != nil {
		return ErrInvalid(err, "Error deleting contact")
	}
	return c.String(http.StatusOK, "Contact deleted")
}

// persondetail renders the detail page for a contact,
// including notes and tags (ready for inline editing).
func (ctrl *controller) persondetail(c echo.Context) error {
	m := ctrl.defaultResponseMap(c, "Contact Details")
	ownerID := c.Get("ownerid").(uint)
	personDB, err := ctrl.model.LoadPerson(c.Param("id"), ownerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrInvalid(err, "Contact not found")
		}
		return ErrInvalid(err, "Error loading contact")
	}
	notes, err := ctrl.model.LoadAllNotesForParent(ownerID, model.ParentTypePerson, personDB.ID)
	if err != nil {
		return ErrInvalid(err, "Could not load notes")
	}

	// Load tags for inline editing (names only)
	tags, err := ctrl.model.ListTagsForParent(ownerID, model.ParentTypePerson, personDB.ID)
	if err != nil {
		return ErrInvalid(err, "Could not load tags")
	}
	tagNames := make([]string, 0, len(tags))
	for _, t := range tags {
		tagNames = append(tagNames, t.Name)
	}

	m["notes"] = notes
	m["right"] = "persondetail"
	m["persondetail"] = personDB
	m["title"] = personDB.Name
	m["ExistingTags"] = tagNames
	m["noteparenttype"] = model.ParentTypePerson

	ctrl.model.TouchRecentView(ownerID, model.EntityPerson, personDB.ID)
	return c.Render(http.StatusOK, "persondetail.html", m)
}

// personedit serves both GET (render edit form) and POST (apply updates).
// On save, ContactInfos use "replace" semantics (delete-all + insert provided set),
// and tags are updated according to the SavePerson tag semantics (nil/empty/non-empty).
func (ctrl *controller) personedit(c echo.Context) error {
	ownerID := c.Get("ownerid").(uint)
	id := c.Param("id")

	switch c.Request().Method {
	case http.MethodGet:
		m := ctrl.defaultResponseMap(c, "Edit Contact")
		personDB, err := ctrl.model.LoadPerson(id, ownerID)
		if err != nil {
			return ErrInvalid(err, "Error loading contact")
		}

		// Load existing tags for prefill (used by Alpine token UI)
		existingTags, err := ctrl.model.ListTagsForParent(ownerID, model.ParentTypePerson, personDB.ID)
		if err != nil {
			return ErrInvalid(err, "Error loading tags")
		}
		tagNames := make([]string, 0, len(existingTags))
		for _, t := range existingTags {
			tagNames = append(tagNames, t.Name)
		}

		m["cancel"] = fmt.Sprintf("/person/%s/%s", id, personDB.Name)
		m["right"] = "personedit"
		m["title"] = personDB.Name
		m["action"] = fmt.Sprintf("/person/edit/%s", id)
		m["submit"] = "Save"
		m["showremove"] = true
		m["persondetail"] = personDB
		m["companies"] = []*model.Company{&personDB.Company} // keep denormalized company for templates
		m["companyid"] = personDB.CompanyID
		m["prefillTags"] = tagNames // template can JSON-encode this into Alpine state

		return c.Render(http.StatusOK, "personedit.html", m)

	case http.MethodPost:
		// Parse form (including arrays like phone[i].* and multiple "tags")
		if err := c.Request().ParseForm(); err != nil {
			return ErrInvalid(err, "Error parsing form data")
		}
		var pf personForm
		dec := form.NewDecoder()
		if err := dec.Decode(&pf, c.Request().Form); err != nil {
			return ErrInvalid(err, "Error decoding form data")
		}

		dbPerson, err := ctrl.model.LoadPerson(id, ownerID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInvalid(err, "Contact not found")
			}
			return ErrInvalid(err, "Error loading contact")
		}

		// Update scalar fields
		dbPerson.Name = strings.TrimSpace(pf.Name)
		dbPerson.EMail = strings.TrimSpace(pf.Email)
		dbPerson.Position = strings.TrimSpace(pf.Funktion)
		dbPerson.CompanyID = pf.Firma

		// (Optional) keep denormalized company on struct to avoid nil derefs in templates
		company, err := ctrl.model.LoadCompany(pf.Firma, ownerID)
		if err != nil {
			return ErrInvalid(err, "Error loading company")
		}
		dbPerson.Company = *company

		// Replace ContactInfos on save: collect provided set (model layer performs delete/insert)
		dbPerson.ContactInfos = []model.ContactInfo{}
		for _, ci := range pf.Phone {
			ci.Type = strings.TrimSpace(ci.Type)
			ci.Label = strings.TrimSpace(ci.Label)
			ci.Value = strings.TrimSpace(ci.Value)
			if ci.Value == "" {
				continue
			}
			if ci.Type == "" {
				ci.Type = "phone"
			}
			dbPerson.ContactInfos = append(dbPerson.ContactInfos, model.ContactInfo{
				Type:       ci.Type,
				Label:      ci.Label,
				Value:      ci.Value,
				OwnerID:    ownerID,
				ParentType: model.ParentTypePerson, // polymorphic discriminator
				// ParentID is set during save
			})
		}

		// Collect tags (multiple inputs name="tags")
		tagNames := normalizeSliceInput(pf.Tags)

		if err := ctrl.model.SavePerson(dbPerson, ownerID, tagNames); err != nil {
			return ErrInvalid(err, "Error saving contact")
		}
		return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/person/%d", dbPerson.ID))
	}
	return nil
}

// personTagsUpdate replaces all tags for a person (by name) transactionally.
// Accepts multiple inputs name="tags" and supports AJAX or classic form POST.
//
// POST /person/:id/tags
func (ctrl *controller) personTagsUpdate(c echo.Context) error {
	ownerID := c.Get("ownerid").(uint)

	// Parse person ID from path
	personID64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return ErrInvalid(err, "Invalid person ID")
	}
	personID := uint(personID64)

	// Parse tags from form (multiple name="tags" inputs)
	if err := c.Request().ParseForm(); err != nil {
		return ErrInvalid(err, "Failed to parse form")
	}
	tagNames := normalizeSliceInput(c.Request().Form["tags"])

	// Replace tags transactionally via model (keep DB logic in the model layer)
	if err := ctrl.model.ReplacePersonTagsByName(personID, ownerID, tagNames); err != nil {
		return ErrInvalid(err, "Error updating tags")
	}

	// AJAX-friendly response (HTMX / XMLHttpRequest)
	if c.Request().Header.Get("HX-Request") != "" || c.Request().Header.Get("X-Requested-With") == "XMLHttpRequest" {
		return c.JSON(http.StatusOK, echo.Map{"status": "ok"})
	}

	// Fallback redirect for standard form submit
	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/person/%d", personID))
}
