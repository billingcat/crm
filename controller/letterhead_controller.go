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

// letterheadInit registers all routes related to letterhead templates.
// These endpoints handle listing, creation from existing PDFs, editing,
// updating regions, and deletion. No direct DB access happens here —
// all persistence is handled via the model layer.
func (ctrl *controller) letterheadInit(e *echo.Echo) {
	g := e.Group("/letterhead", ctrl.authMiddleware)
	g.GET("", ctrl.letterheadList)
	g.GET("/new", ctrl.letterheadNewForm)
	g.POST("/new", ctrl.letterheadCreateFromExisting) // upload PDF → render PNG previews → create template via model
	g.GET("/:id/edit", ctrl.letterheadEdit)           // open the editor (ensures 3 fixed regions exist)
	g.POST("/:id/regions", ctrl.letterheadSave)       // update regions (via model)
	g.POST("/:id/delete", ctrl.letterheadDelete)
	g.GET("/:id/fonts", ctrl.listTemplateFonts, ctrl.mustBeOwnerOfTemplate("id"))
}

// GET /letterhead
// Lists all letterhead templates for the current owner.
func (ctrl *controller) letterheadList(c echo.Context) error {
	ownerID := c.Get("ownerid").(uint)

	list, err := ctrl.model.ListLetterheadTemplates(ownerID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Could not load letterheads")
	}

	m := ctrl.defaultResponseMap(c, "Letterheads")
	m["Templates"] = list
	return c.Render(http.StatusOK, "letterhead_list.html", m)
}

// GET /letterhead/new
// Displays existing PDF files from the owner's asset directory as selectable sources
// for creating new letterhead templates.
func (ctrl *controller) letterheadNewForm(c echo.Context) error {
	ownerID := c.Get("ownerid").(uint)

	root := ctrl.userAssetsDir(ownerID)
	files, _ := ctrl.listPDFFiles(root)

	m := ctrl.defaultResponseMap(c, "New Letterhead")
	m["Files"] = files
	m["HasFiles"] = len(files) > 0
	m["FileManagerURL"] = "/filemanager" // adjust if you have a different route

	return c.Render(http.StatusOK, "letterhead_new.html", m)
}

// POST /letterhead/new
// Creates a new letterhead template from an existing PDF located in the owner's asset directory.
// No direct DB operations occur here; model.SaveLetterheadTemplate handles persistence.
func (ctrl *controller) letterheadCreateFromExisting(c echo.Context) error {
	ownerID := c.Get("ownerid").(uint)

	name := strings.TrimSpace(c.FormValue("name"))
	relPath := strings.TrimSpace(c.FormValue("path")) // relative to the owner’s asset directory
	if relPath == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Please select a PDF file.")
	}

	root := ctrl.userAssetsDir(ownerID)
	abs, err := safeJoin(root, relPath)
	if err != nil {
		return err
	}
	if !strings.EqualFold(filepath.Ext(abs), ".pdf") {
		return echo.NewHTTPError(http.StatusBadRequest, "Only PDF files are allowed.")
	}

	if name == "" {
		name = strings.TrimSuffix(filepath.Base(abs), filepath.Ext(abs))
	}

	// Create a model record (without direct DB logic in controller)
	tpl := &model.LetterheadTemplate{
		OwnerID: ownerID,
		Name:    name,
		PDFPath: relPath, // store relative to owner's asset directory
	}
	if err := ctrl.model.SaveLetterheadTemplate(tpl, ownerID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError,
			fmt.Sprintf("Could not create letterhead: %v", err))
	}

	// Extract preview images and page size from the PDF
	if w, h, url1, url2, err := ctrl.ensureLetterheadPreviews(ownerID, tpl); err == nil {
		_ = ctrl.model.UpdateLetterheadPageSize(tpl.ID, ownerID, w, h)
		_ = ctrl.model.UpdateLetterheadPreviewURLs(tpl.ID, ownerID, url1, url2)
		_ = ctrl.model.EnsureDefaultLetterheadRegions(tpl.ID, ownerID, w, h)
	} else {
		// Fallback to A4 defaults if preview generation fails
		_ = ctrl.model.UpdateLetterheadPageSize(tpl.ID, ownerID, 21.0, 29.7)
		_ = ctrl.model.EnsureDefaultLetterheadRegions(tpl.ID, ownerID, 21.0, 29.7)
	}

	// Redirect to the editor view for the new template
	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/letterhead/%d/edit", tpl.ID))
}

// GET /letterhead/:id/edit
// Loads the letterhead editor, ensuring that preview images and the
// three fixed editable regions (sender, address, footer) exist.
func (ctrl *controller) letterheadEdit(c echo.Context) error {
	ownerID := c.Get("ownerid").(uint)
	id, err := parseUintParam(c, "id")
	if err != nil || id == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid ID")
	}

	tpl, err := ctrl.model.LoadLetterheadTemplate(id, ownerID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Letterhead not found")
	}

	// Ensure previews and default regions exist
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

	m := ctrl.defaultResponseMap(c, "Edit Letterhead")
	m["Template"] = tpl
	return c.Render(http.StatusOK, "letterhead_editor.html", m)
}

// POST /letterhead/:id/regions
// Updates only the three fixed regions of a letterhead (sender, address, footer).
// Expects JSON payload:
//
//	{ "regions": [ { kind:"sender", xCm:..., ... }, ... ] }
//
// Additionally handles optional font assignments and page size.
func (ctrl *controller) letterheadSave(c echo.Context) error {
	ownerID := c.Get("ownerid").(uint)
	id, err := parseUintParam(c, "id")
	if err != nil || id == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid ID")
	}

	var payload struct {
		PageWidthCm  float64   `json:"page_width_cm"`
		PageHeightCm float64   `json:"page_height_cm"`
		Fonts        *struct { // optional
			Normal string `json:"normal"`
			Bold   string `json:"bold"`
			Italic string `json:"italic"`
		} `json:"fonts"`
		Regions []model.PlacedRegion `json:"regions"`
	}
	if err := c.Bind(&payload); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request payload")
	}

	// Enforce ownership and set foreign keys for integrity
	for i := range payload.Regions {
		payload.Regions[i].TemplateID = uint(id)
		payload.Regions[i].OwnerID = ownerID
	}

	// Optional font validation (only extension check)
	validateExt := func(name string) (string, error) {
		if name == "" {
			return "", nil
		}
		ext := strings.ToLower(path.Ext(name))
		if ext != ".ttf" && ext != ".otf" {
			return "", fmt.Errorf("unsupported font type: %s", ext)
		}
		return name, nil
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

	// Save fonts, page size, and region layout atomically in one transaction.
	if err := ctrl.model.UpdateLetterheadRegionsAndFonts(
		uint(id), ownerID, payload.Regions, fonts,
		payload.PageWidthCm, payload.PageHeightCm,
	); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Save failed: %v", err))
	}

	return c.JSON(http.StatusOK, map[string]any{"status": "ok"})
}

// POST /letterhead/:id/delete
// Deletes a letterhead template and its associated preview files.
// Deletion in DB triggers cascading removal of its regions.
func (ctrl *controller) letterheadDelete(c echo.Context) error {
	ownerID := c.Get("ownerid").(uint)
	id, err := parseUintParam(c, "id")
	if err != nil || id == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid ID")
	}

	// 1) Delete DB record (CASCADE removes regions)
	if err := ctrl.model.DeleteLetterheadTemplate(id, ownerID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Deletion failed: %v", err))
	}

	// 2) Remove preview files under /uploads (best-effort)
	previewsDir := filepath.Join(
		ctrl.uploadsDir(), "letterhead",
		fmt.Sprintf("owner%d", ownerID),
		fmt.Sprintf("%d", id),
	)
	_ = os.RemoveAll(previewsDir)

	// Optional: flash message “Deleted successfully” if you have flash helpers
	// ctrl.addFlash(c, "success", "Letterhead deleted")

	return c.Redirect(http.StatusSeeOther, "/letterhead")
}
