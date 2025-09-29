package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/billingcat/crm/model"

	"github.com/go-playground/form/v4"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

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
}

type personForm struct {
	Name     string            `form:"name"`
	Firma    int               `form:"firma"`
	Email    string            `form:"email"`
	Funktion string            `form:"funktion"`
	Phone    []contactInfoForm `form:"phone"` // neuer Satz Felder: type/label/value
}

func (ctrl *controller) personnew(c echo.Context) error {
	m := ctrl.defaultResponseMap(c, "Neuen Kontakt anlegen")
	ownerID := c.Get("ownerid").(uint)

	switch c.Request().Method {
	case http.MethodGet:
		if cmpyID := c.Param("company"); cmpyID != "" {
			cmpy, err := ctrl.model.LoadCompany(cmpyID, ownerID)
			if err != nil {
				return ErrInvalid(err, "Fehler beim Laden der Firma")
			}
			m["companies"] = []*model.Company{cmpy}
		} else {
			var err error
			m["companies"], err = ctrl.model.LoadAllCompanies(ownerID)
			if err != nil {
				return ErrInvalid(err, "Fehler beim Laden der Firmen")
			}
		}
		m["persondetail"] = &model.Person{}
		m["action"] = "/person/new"
		m["submit"] = "Kontakt anlegen"
		m["cancel"] = "/"
		m["showremove"] = false
		return c.Render(http.StatusOK, "personedit.html", m)

	case http.MethodPost:
		// Form dekodieren (inkl. Arrays: phone[i].type/label/value)
		if err := c.Request().ParseForm(); err != nil {
			return ErrInvalid(err, "Fehler beim Verarbeiten der Formulardaten")
		}
		var pf personForm
		dec := form.NewDecoder()
		if err := dec.Decode(&pf, c.Request().Form); err != nil {
			return ErrInvalid(err, "Fehler beim Verarbeiten der Formulardaten")
		}

		personDB := model.Person{
			Name:      strings.TrimSpace(pf.Name),
			EMail:     strings.TrimSpace(pf.Email),
			Position:  strings.TrimSpace(pf.Funktion),
			CompanyID: pf.Firma,
			OwnerID:   ownerID,
		}

		// Kontaktinfos übernehmen
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
				ParentType: "person", // wichtig: gehört zur Person
				// ParentID setzt GORM beim Assoc-Save
			})
		}

		if err := ctrl.model.CreatePerson(&personDB); err != nil {
			return ErrInvalid(err, "Fehler beim Anlegen des Kontakts")
		}
		return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/person/%d", personDB.ID))
	}
	return nil
}

func (ctrl *controller) deletePersonWithID(c echo.Context) error {
	ownerID := c.Get("ownerid").(uint)
	personID := c.Param("id")
	personDB, err := ctrl.model.LoadPerson(personID, ownerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrInvalid(err, "Kontakt nicht gefunden")
		}
	}
	if personDB.OwnerID != ownerID {
		return echo.ErrForbidden
	}
	if err := ctrl.model.RemovePerson(personID, ownerID); err != nil {
		return ErrInvalid(err, "Fehler beim Löschen des Kontakts")
	}
	return c.String(http.StatusOK, "Kontakt gelöscht")
}

func (ctrl *controller) persondetail(c echo.Context) error {
	m := ctrl.defaultResponseMap(c, "Kontakt-Details")
	ownerID := c.Get("ownerid").(uint)
	personDB, err := ctrl.model.LoadPerson(c.Param("id"), ownerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrInvalid(err, "Kontakt nicht gefunden")
		}
		return ErrInvalid(err, "Fehler beim Laden des Kontakts")
	}
	notes, err := ctrl.model.LoadAllNotesForParent(ownerID, "people", personDB.ID)
	if err != nil {
		return ErrInvalid(err, "Kann Notizen nicht laden")
	}
	m["notes"] = notes
	m["right"] = "persondetail"
	m["persondetail"] = personDB
	m["title"] = personDB.Name
	ctrl.model.TouchRecentView(ownerID, model.EntityPerson, personDB.ID)
	return c.Render(http.StatusOK, "persondetail.html", m)
}

func (ctrl *controller) personedit(c echo.Context) error {
	ownerID := c.Get("ownerid").(uint)
	id := c.Param("id")

	switch c.Request().Method {
	case http.MethodGet:
		m := ctrl.defaultResponseMap(c, "Kontakt bearbeiten")
		personDB, err := ctrl.model.LoadPerson(id, ownerID)
		if err != nil {
			return ErrInvalid(err, "Fehler beim Laden des Kontakts")
		}
		m["cancel"] = fmt.Sprintf("/person/%s/%s", id, personDB.Name)
		m["right"] = "personedit"
		m["title"] = personDB.Name
		m["action"] = fmt.Sprintf("/person/edit/%s", id)
		m["submit"] = "Speichern"
		m["showremove"] = true
		m["persondetail"] = personDB
		m["companies"] = []*model.Company{&personDB.Company}
		m["companyid"] = personDB.CompanyID
		return c.Render(http.StatusOK, "personedit.html", m)

	case http.MethodPost:
		if err := c.Request().ParseForm(); err != nil {
			return ErrInvalid(err, "Fehler beim Verarbeiten der Formulardaten")
		}
		var pf personForm
		dec := form.NewDecoder()
		if err := dec.Decode(&pf, c.Request().Form); err != nil {
			return ErrInvalid(err, "Fehler beim Verarbeiten der Formulardaten")
		}

		dbPerson, err := ctrl.model.LoadPerson(id, ownerID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInvalid(err, "Kontakt nicht gefunden")
			}
			return ErrInvalid(err, "Fehler beim Laden des Kontakts")
		}

		dbPerson.Name = strings.TrimSpace(pf.Name)
		dbPerson.EMail = strings.TrimSpace(pf.Email)
		dbPerson.Position = strings.TrimSpace(pf.Funktion)
		dbPerson.CompanyID = pf.Firma

		company, err := ctrl.model.LoadCompany(pf.Firma, ownerID)
		if err != nil {
			return ErrInvalid(err, "Fehler beim Laden der Firma")
		}
		dbPerson.Company = *company

		// Kontaktinfos ersetzen (einfacher Weg)
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
				ParentType: "person", // wichtig
			})
		}

		if err := ctrl.model.SavePerson(dbPerson, ownerID); err != nil {
			return ErrInvalid(err, "Fehler beim Speichern des Kontakts")
		}
		return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/person/%d", dbPerson.ID))
	}
	return nil
}
