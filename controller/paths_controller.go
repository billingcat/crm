// controller/paths.go
package controller

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// base directory for public uploads (Echo: e.Static("/uploads", "uploads")).
// We assume ctrl.cfg.BaseDir is the project root where the "uploads" folder is located.
func (ctrl *controller) uploadsDir() string {
	return filepath.Join(ctrl.model.Config.Basedir, "uploads")
}

// uploadsAbsToURL changes an absolute path (under uploadsDir) into a public URL
// /uploads/<rel>.
func (ctrl *controller) uploadsAbsToURL(abs string) (string, error) {
	root := ctrl.uploadsDir()
	rootAbs, _ := filepath.Abs(root)
	absFile, _ := filepath.Abs(abs)

	// Safety: absFile muss unter uploads liegen
	if !strings.HasPrefix(absFile, rootAbs+string(os.PathSeparator)) && absFile != rootAbs {
		return "", errors.New("file not under uploads root")
	}
	rel, err := filepath.Rel(root, absFile)
	if err != nil {
		return "", err
	}
	return "/uploads/" + filepath.ToSlash(rel), nil
}
