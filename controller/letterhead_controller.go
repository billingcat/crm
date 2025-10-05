package controller

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/billingcat/crm/model" // adjust to your module path
	"github.com/labstack/echo/v4"
)

// letterheadInit wires letterhead routes (no DB calls here).
func (ctrl *controller) letterheadInit(e *echo.Echo) {
	g := e.Group("/letterhead", ctrl.authMiddleware)
	g.GET("", ctrl.letterheadList)
	g.GET("/new", ctrl.letterheadNewForm)
	g.POST("/new", ctrl.letterheadCreateFromExisting) // upload PDF -> render PNGs -> create template (via model)
	g.GET("/:id/edit", ctrl.letterheadEdit)           // load editor (ensures three fixed regions exist)
	g.POST("/:id/regions", ctrl.letterheadSave)       // update only the three fixed regions (via model)
	g.POST("/:id/delete", ctrl.letterheadDelete)
	g.GET("/:id/fonts", ctrl.listTemplateFonts, ctrl.mustBeOwnerOfTemplate("id"))

}

// GET /letterhead
func (ctrl *controller) letterheadList(c echo.Context) error {
	ownerID := c.Get("ownerid").(uint)

	list, err := ctrl.model.ListLetterheadTemplates(ownerID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Konnte Briefbögen nicht laden")
	}

	m := ctrl.defaultResponseMap(c, "Briefbögen")
	m["Templates"] = list
	return c.Render(http.StatusOK, "letterhead_list.html", m)
}

// GET /letterhead/new
// Zeigt vorhandene PDF-Dateien aus dem Owner-Assets-Verzeichnis als Auswahl.
func (ctrl *controller) letterheadNewForm(c echo.Context) error {
	ownerID := c.Get("ownerid").(uint)

	root := ctrl.userAssetsDir(ownerID)
	files, _ := ctrl.listPDFFiles(root)

	m := ctrl.defaultResponseMap(c, "Neuer Briefbogen")
	m["Files"] = files
	m["HasFiles"] = len(files) > 0
	m["FileManagerURL"] = "/filemanager" // ggf. anpassen

	return c.Render(http.StatusOK, "letterhead_new.html", m)
}

// POST /letterhead/new
// Legt ein Template aus einer bestehenden PDF im Owner-Verzeichnis an und leitet zum Editor.
// Keine DB-Calls hier; Speicherung via ctrl.model.SaveLetterheadTemplate.
func (ctrl *controller) letterheadCreateFromExisting(c echo.Context) error {
	ownerID := c.Get("ownerid").(uint)

	name := strings.TrimSpace(c.FormValue("name"))
	relPath := strings.TrimSpace(c.FormValue("path")) // relativ zum Owner-Assets-Verzeichnis
	if relPath == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Bitte eine PDF-Datei auswählen.")
	}

	root := ctrl.userAssetsDir(ownerID)
	abs, err := safeJoin(root, relPath)
	if err != nil {
		return err
	}
	if !strings.EqualFold(filepath.Ext(abs), ".pdf") {
		return echo.NewHTTPError(http.StatusBadRequest, "Nur PDF-Dateien sind erlaubt.")
	}

	if name == "" {
		name = strings.TrimSuffix(filepath.Base(abs), filepath.Ext(abs))
	}

	// Anlegen über Model-API (ohne DB im Controller)
	tpl := &model.LetterheadTemplate{
		OwnerID: ownerID,
		Name:    name,
		PDFPath: relPath, // relativ zum Owner-Verzeichnis speichern
	}
	if err := ctrl.model.SaveLetterheadTemplate(tpl, ownerID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError,
			fmt.Sprintf("Konnte Briefbogen nicht anlegen: %v", err))
	}
	// Previews & Seitengröße aus PDF holen
	if w, h, url1, url2, err := ctrl.ensureLetterheadPreviews(ownerID, tpl); err == nil {
		_ = ctrl.model.UpdateLetterheadPageSize(tpl.ID, ownerID, w, h)
		_ = ctrl.model.UpdateLetterheadPreviewURLs(tpl.ID, ownerID, url1, url2)
		_ = ctrl.model.EnsureDefaultLetterheadRegions(tpl.ID, ownerID, w, h)
	} else {
		_ = ctrl.model.UpdateLetterheadPageSize(tpl.ID, ownerID, 21.0, 29.7)
		_ = ctrl.model.EnsureDefaultLetterheadRegions(tpl.ID, ownerID, 21.0, 29.7)
	}

	// >>> WICHTIG: Redirect zum Editor gemäß deiner Route
	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/letterhead/%d/edit", tpl.ID))
}

func (ctrl *controller) letterheadEdit(c echo.Context) error {
	ownerID := c.Get("ownerid").(uint)
	id, err := parseUintParam(c, "id")
	if err != nil || id == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Ungültige ID")
	}

	tpl, err := ctrl.model.LoadLetterheadTemplate(id, ownerID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Briefbogen nicht gefunden")
	}

	if tpl.PageWidthCm <= 0 || tpl.PageHeightCm <= 0 || tpl.PreviewPage1URL == "" {
		if w, h, url1, url2, e := ctrl.ensureLetterheadPreviews(ownerID, tpl); e == nil {
			_ = ctrl.model.UpdateLetterheadPageSize(tpl.ID, ownerID, w, h)
			_ = ctrl.model.UpdateLetterheadPreviewURLs(tpl.ID, ownerID, url1, url2)
			_ = ctrl.model.EnsureDefaultLetterheadRegions(tpl.ID, ownerID, w, h)
			tpl, _ = ctrl.model.LoadLetterheadTemplate(id, ownerID)
		} else {
			if tpl.PageWidthCm <= 0 || tpl.PageHeightCm <= 0 {
				_ = ctrl.model.UpdateLetterheadPageSize(tpl.ID, ownerID, 21.0, 29.7)
			}
			_ = ctrl.model.EnsureDefaultLetterheadRegions(tpl.ID, ownerID, 21.0, 29.7)
		}
	}
	m := ctrl.defaultResponseMap(c, "Briefbogen bearbeiten")
	m["Template"] = tpl
	return c.Render(http.StatusOK, "letterhead_editor.html", m)
}

// POST /letterhead/:id/regions
// Nimmt NUR die drei festen Regionen entgegen und speichert sie über die Model-API.
// Erwartet JSON-Payload: { "regions": [ {kind:"sender", xCm:.., ...}, ... ] }
func (ctrl *controller) letterheadSave(c echo.Context) error {
	ownerID := c.Get("ownerid").(uint)
	id, err := parseUintParam(c, "id")
	if err != nil || id == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Ungültige ID")
	}

	var payload struct {
		PageWidthCm  float64   `json:"page_width_cm"`
		PageHeightCm float64   `json:"page_height_cm"`
		Fonts        *struct { // optional, wenn nichts gewählt
			Normal string `json:"normal"`
			Bold   string `json:"bold"`
			Italic string `json:"italic"`
		} `json:"fonts"`
		Regions []model.PlacedRegion `json:"regions"`
	}
	if err := c.Bind(&payload); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Ungültige Daten")
	}

	// Ownership & TemplateID anreichern (Integrität)
	for i := range payload.Regions {
		payload.Regions[i].TemplateID = uint(id)
		payload.Regions[i].OwnerID = ownerID
	}

	// (Optional) Font-Dateinamen leicht validieren (nur Extension).
	// Existenzprüfung kannst du machen, wenn du ctrl.userAssetsDir(ownerID) hast.
	validateExt := func(name string) (string, error) {
		if name == "" {
			return "", nil
		}
		ext := strings.ToLower(path.Ext(name))
		if ext != ".ttf" && ext != ".otf" {
			return "", fmt.Errorf("unsupported font type: %s", ext)
		}
		return name, nil // Basename speichern
	}
	var fonts *model.TemplateFonts
	if payload.Fonts != nil {
		n, err := validateExt(payload.Fonts.Normal)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		b, err := validateExt(payload.Fonts.Bold)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		i, err := validateExt(payload.Fonts.Italic)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		fonts = &model.TemplateFonts{Normal: n, Bold: b, Italic: i}
	}

	// *** NEU: Template-Meta (Fonts + PageSize) + Regions in EINER Transaktion speichern
	if err := ctrl.model.UpdateLetterheadRegionsAndFonts(uint(id), ownerID, payload.Regions, fonts, payload.PageWidthCm, payload.PageHeightCm); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Speichern fehlgeschlagen: %v", err))
	}

	return c.JSON(http.StatusOK, map[string]any{"status": "ok"})
}

func (ctrl *controller) letterheadDelete(c echo.Context) error {
	ownerID := c.Get("ownerid").(uint)
	id, err := parseUintParam(c, "id")
	if err != nil || id == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Ungültige ID")
	}

	// 1) DB-Datensatz via Model-API löschen (CASCADE entfernt Regionen)
	if err := ctrl.model.DeleteLetterheadTemplate(id, ownerID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Löschen fehlgeschlagen: %v", err))
	}

	// 2) Preview-Dateien unter /uploads entfernen (best-effort)
	previewsDir := filepath.Join(
		ctrl.uploadsDir(), "letterhead",
		fmt.Sprintf("owner%d", ownerID),
		fmt.Sprintf("%d", id),
	)
	_ = os.RemoveAll(previewsDir)

	// Optional: Flash "Erfolgreich gelöscht" setzen, falls du bereits Flash-Helper hast
	// ctrl.addFlash(c, "success", "Briefbogen gelöscht")

	return c.Redirect(http.StatusSeeOther, "/letterhead")
}
