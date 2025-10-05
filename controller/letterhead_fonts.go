// controller/letterhead_fonts.go

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

type FontFile struct {
	Filename string `json:"filename"` // basename only
}

const ctxTemplateKey = "letterhead_template"

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

			// Nur Model-Funktionen aufrufen – keine DB-Queries hier.
			var tpl *model.LetterheadTemplate
			if tpl, err = ctrl.model.LoadLetterheadTemplateForAccess(tplID, ownerID, isAdmin); err != nil {
				// 404 bei nicht gefunden, sonst 500
				if err == gorm.ErrRecordNotFound {
					return echo.NewHTTPError(http.StatusNotFound, "template not found")
				}
				return echo.NewHTTPError(http.StatusInternalServerError, "db error: "+err.Error())
			}

			// Sicherheitsgurt: Wenn kein Admin, aber fremder Owner → 403
			if !isAdmin && tpl.OwnerID != ownerID {
				return echo.NewHTTPError(http.StatusForbidden, "forbidden")
			}

			c.Set(ctxTemplateKey, tpl)
			return next(c)
		}
	}
}

// Helper, um das Template im Handler wieder zu bekommen
func TemplateFromContext(c echo.Context) *model.LetterheadTemplate {
	if v := c.Get(ctxTemplateKey); v != nil {
		if tpl, ok := v.(*model.LetterheadTemplate); ok {
			return tpl
		}
	}
	return nil
}
