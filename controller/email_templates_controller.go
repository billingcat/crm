package controller

import (
	"net/http"

	"github.com/billingcat/crm/model"
	"github.com/labstack/echo/v4"
)

func (ctrl *controller) emailTemplatesInit(e *echo.Echo) {
	g := e.Group("/email-templates")
	g.Use(ctrl.authMiddleware)
	g.GET("", ctrl.emailTemplatesList)
	g.GET("/edit/:kind", ctrl.emailTemplateEdit)
	g.POST("/edit/:kind", ctrl.emailTemplateSave)
}

// emailTemplateKindInfo describes a configurable mail kind for the UI.
type emailTemplateKindInfo struct {
	Kind             model.EmailTemplateKind
	Title            string
	Description      string
	DefaultSubject   string
	DefaultBody      string
	Placeholders     []emailTemplatePlaceholder
	HasCustomization bool
}

type emailTemplatePlaceholder struct {
	Name string
	Desc string
}

// emailTemplateKinds returns the list of known mail kinds shown in the UI.
// Add new entries here when a new mail kind becomes configurable.
func emailTemplateKinds() []emailTemplateKindInfo {
	return []emailTemplateKindInfo{
		{
			Kind:           model.EmailTemplateKindInvoice,
			Title:          "Rechnung",
			Description:    "Vorbefüllter Mail-Text für „Rechnung per E-Mail senden“ auf der Rechnungs-Detailseite.",
			DefaultSubject: model.DefaultInvoiceMailSubject,
			DefaultBody:    model.DefaultInvoiceMailBody,
			Placeholders:   invoicePlaceholders(),
		},
	}
}

func invoicePlaceholders() []emailTemplatePlaceholder {
	out := make([]emailTemplatePlaceholder, 0, len(model.InvoiceMailPlaceholders))
	for _, p := range model.InvoiceMailPlaceholders {
		out = append(out, emailTemplatePlaceholder{Name: p.Name, Desc: p.Desc})
	}
	return out
}

func findEmailTemplateKind(kind string) (emailTemplateKindInfo, bool) {
	for _, k := range emailTemplateKinds() {
		if string(k.Kind) == kind {
			return k, true
		}
	}
	return emailTemplateKindInfo{}, false
}

func (ctrl *controller) emailTemplatesList(c echo.Context) error {
	ownerID := c.Get("ownerid").(uint)
	kinds := emailTemplateKinds()
	for i := range kinds {
		t, err := ctrl.model.LoadOwnerEmailTemplate(ownerID, kinds[i].Kind)
		if err != nil {
			return ErrInvalid(err, "Kann E-Mail-Vorlagen nicht laden")
		}
		kinds[i].HasCustomization = t != nil
	}
	m := ctrl.defaultResponseMap(c, "E-Mail-Vorlagen")
	m["kinds"] = kinds
	return c.Render(http.StatusOK, "email_templates_list.html", m)
}

func (ctrl *controller) emailTemplateEdit(c echo.Context) error {
	ownerID := c.Get("ownerid").(uint)
	info, ok := findEmailTemplateKind(c.Param("kind"))
	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, "Unbekannte Vorlage")
	}

	t, err := ctrl.model.LoadOwnerEmailTemplate(ownerID, info.Kind)
	if err != nil {
		return ErrInvalid(err, "Kann E-Mail-Vorlage nicht laden")
	}

	subject := info.DefaultSubject
	body := info.DefaultBody
	if t != nil {
		if t.Subject != "" {
			subject = t.Subject
		}
		if t.Body != "" {
			body = t.Body
		}
	}

	m := ctrl.defaultResponseMap(c, "E-Mail-Vorlage: "+info.Title)
	m["info"] = info
	m["subject"] = subject
	m["body"] = body
	m["hasCustomization"] = t != nil
	return c.Render(http.StatusOK, "email_template_edit.html", m)
}

func (ctrl *controller) emailTemplateSave(c echo.Context) error {
	ownerID := c.Get("ownerid").(uint)
	info, ok := findEmailTemplateKind(c.Param("kind"))
	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, "Unbekannte Vorlage")
	}

	t := &model.EmailTemplate{
		OwnerID: ownerID,
		Kind:    info.Kind,
		Subject: c.FormValue("subject"),
		Body:    c.FormValue("body"),
	}
	if err := ctrl.model.SaveEmailTemplate(t); err != nil {
		return ErrInvalid(err, "Kann E-Mail-Vorlage nicht speichern")
	}
	_ = AddFlash(c, "success", "E-Mail-Vorlage gespeichert.")
	return c.Redirect(http.StatusSeeOther, "/email-templates")
}
