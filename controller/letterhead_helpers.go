// controller/letterhead_helpers.go
package controller

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LetterheadOption is used to render selectable PDFs in the form.
type LetterheadOption struct {
	Name      string // filename only, e.g. "briefbogen.pdf"
	RelPath   string // path relative to the user assets dir, e.g. "branding/briefbogen.pdf"
	ModTime   time.Time
	SizeHuman string
}

// userAssetsDir returns the absolute directory for a given owner.
// Example: <basedir>/assets/userassets/owner<ownerID>
func (ctrl *controller) userAssetsDir(ownerID uint) string {
	return filepath.Join(ctrl.model.Config.Basedir, "assets", "userassets", fmt.Sprintf("owner%d", ownerID))
}

// listPDFFiles returns all *.pdf files in the user assets dir (recursive).
func (ctrl *controller) listPDFFiles(root string) ([]LetterheadOption, error) {
	var out []LetterheadOption
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors silently to avoid breaking the page
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(d.Name()), ".pdf") {
			info, err := d.Info()
			if err != nil {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return nil
			}
			out = append(out, LetterheadOption{
				Name:      d.Name(),
				RelPath:   filepath.ToSlash(rel), // keep URLs nice
				ModTime:   info.ModTime(),
				SizeHuman: humanSize(info.Size()),
			})
		}
		return nil
	})
	return out, err
}
