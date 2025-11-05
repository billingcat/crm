package controller

import (
	"net/http"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/billingcat/crm/model"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// FontFile represents a single available font file within a user's asset directory.
type FontFile struct {
	Filename string `json:"filename"` // basename only, e.g. "Roboto-Regular.ttf"
}

const ctxTemplateKey = "letterhead_template"

// listTemplateFonts returns all available .ttf and .otf font files for the current owner.
// Fonts are read from the user's asset directory and sorted alphabetically.
func (ctrl *controller) listTemplateFonts(c echo.Context) error {
	ownerID := c.Get("ownerid").(uint)
	dir := ctrl.userAssetsDir(ownerID)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "list fonts: "+err.Error())
	}

	var out []FontFile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := strings.ToLower(path.Ext(name))
		if ext == ".ttf" || ext == ".otf" {
			out = append(out, FontFile{Filename: name})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Filename < out[j].Filename })
	return c.JSON(http.StatusOK, out)
}

// mustBeOwnerOfTemplate is a middleware ensuring that the current user is either:
//   - the owner of the requested letterhead template, or
//   - an administrator.
//
// The middleware:
//  1. Extracts the template ID from the route parameter.
//  2. Loads the template via the model layer (no direct DB calls).
//  3. Validates ownership unless the user has admin privileges.
//  4. Stores the loaded template in the Echo context for downstream handlers.
func (ctrl *controller) mustBeOwnerOfTemplate(paramName string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ownerID, ok := c.Get("ownerid").(uint)
			if !ok || ownerID == 0 {
				return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
			}
			isAdmin, _ := c.Get("is_admin").(bool)

			idStr := c.Param(paramName)
			idU64, err := strconv.ParseUint(idStr, 10, 64)
			if err != nil || idU64 == 0 {
				return echo.NewHTTPError(http.StatusBadRequest, "invalid template id")
			}
			tplID := uint(idU64)

			// Call model-layer access function (no raw DB calls in controller).
			var tpl *model.LetterheadTemplate
			if tpl, err = ctrl.model.LoadLetterheadTemplateForAccess(tplID, ownerID, isAdmin); err != nil {
				if err == gorm.ErrRecordNotFound {
					return echo.NewHTTPError(http.StatusNotFound, "template not found")
				}
				return echo.NewHTTPError(http.StatusInternalServerError, "db error: "+err.Error())
			}

			// Extra guard: if not admin but owner mismatch → deny access
			if !isAdmin && tpl.OwnerID != ownerID {
				return echo.NewHTTPError(http.StatusForbidden, "forbidden")
			}

			c.Set(ctxTemplateKey, tpl)
			return next(c)
		}
	}
}

// TemplateFromContext retrieves the loaded LetterheadTemplate (if available)
// from the Echo context. This is set by the mustBeOwnerOfTemplate middleware.
func TemplateFromContext(c echo.Context) *model.LetterheadTemplate {
	if v := c.Get(ctxTemplateKey); v != nil {
		if tpl, ok := v.(*model.LetterheadTemplate); ok {
			return tpl
		}
	}
	return nil
}
