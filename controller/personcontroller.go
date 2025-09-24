package controller

import (
	"errors"
	"fmt"
	"net/http"

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

type person struct {
	Name     string      `form:"name"`
	Firma    int         `form:"firma"`
	Email    string      `form:"email"`
	Funktion string      `form:"funktion"`
	Phone    []phoneForm `form:"phone"`
}

func (ctrl *controller) personnew(c echo.Context) error {
	m := ctrl.defaultResponseMap(c, "Neuen Kontakt anlegen")
	ownerID := c.Get("ownerid")
	var err error
	switch c.Request().Method {
	case http.MethodGet:
		if cmpyID := c.Param("company"); cmpyID != "" {
			cmpy, err := ctrl.model.LoadCompany(cmpyID, ownerID)
			if err != nil {
				return ErrInvalid(err, "Fehler beim Laden der Firma")
			}
			m["companies"] = []*model.Company{cmpy}
		} else {
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
		cp := new(person)
		if err := c.Bind(cp); err != nil {
			return ErrInvalid(err, "Fehler beim Verarbeiten der Formulardaten")
		}
		personDB := model.Person{
			Name:      cp.Name,
			EMail:     cp.Email,
			Position:  cp.Funktion,
			CompanyID: cp.Firma,
			OwnerID:   ownerID.(uint),
		}

		if err = ctrl.model.CreatePerson(&personDB); err != nil {
			return ErrInvalid(err, "Fehler beim Anlegen des Kontakts")
		}
		return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/person/%d", personDB.ID))

	}
	return nil
}

func (ctrl *controller) deletePersonWithID(c echo.Context) error {
	ownerID := c.Get("ownerid").(uint)
	personID := c.Param("id")
	// load the person to check if it exists
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
		persondB, err := ctrl.model.LoadPerson(id, ownerID)
		if err != nil {
			return ErrInvalid(err, "Fehler beim Laden des Kontakts")
		}
		m["cancel"] = fmt.Sprintf("/person/%s/%s", id, persondB.Name)
		m["right"] = "personedit"
		m["title"] = persondB.Name
		m["action"] = fmt.Sprintf("/person/edit/%s", id)
		m["submit"] = "Speichern"
		m["showremove"] = true
		m["persondetail"] = persondB
		m["companies"] = []*model.Company{&persondB.Company}
		m["companyid"] = persondB.CompanyID
		return c.Render(http.StatusOK, "personedit.html", m)
	case http.MethodPost:
		m := ctrl.defaultResponseMap(c, "Kontakt bearbeiten")
		var err error
		var cp person
		err = c.Request().ParseForm()
		if err != nil {
			return ErrInvalid(err, "Fehler beim Verarbeiten der Formulardaten")
		}
		dec := form.NewDecoder()
		err = dec.Decode(&cp, c.Request().Form)
		if err != nil {
			return ErrInvalid(err, "Fehler beim Verarbeiten der Formulardaten")
		}

		dbPerson, err := ctrl.model.LoadPerson(id, ownerID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInvalid(err, "Kontakt nicht gefunden")
			}
			return ErrInvalid(err, "Fehler beim Laden des Kontakts")
		}

		dbPerson.Name = cp.Name
		dbPerson.EMail = cp.Email
		dbPerson.Position = cp.Funktion
		dbPerson.CompanyID = cp.Firma
		company, err := ctrl.model.LoadCompany(cp.Firma, ownerID)
		if err != nil {
			return ErrInvalid(err, "Fehler beim Laden der Firma")
		}
		dbPerson.Company = *company
		dbPerson.Phones = []model.Phone{}

		for _, ph := range cp.Phone {
			if ph.Number == "" || ph.Location == "" {
				continue
			}
			dbPhone := model.Phone{
				Number:     ph.Number,
				Location:   ph.Location,
				OwnerID:    ownerID,
				ParentID:   int(dbPerson.ID),
				ParentType: "company",
			}
			dbPerson.Phones = append(dbPerson.Phones, dbPhone)
		}
		if err := ctrl.model.SavePerson(dbPerson, ownerID); err != nil {
			return ErrInvalid(err, "Fehler beim Speichern des Kontakts")
		}
		m["persondetail"] = dbPerson
		return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/person/%d", dbPerson.ID))
	}

	return nil
}
