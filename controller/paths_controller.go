// controller/paths.go
package controller

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Basisverzeichnis für öffentliche Uploads (Echo: e.Static("/uploads", "uploads")).
// Hier gehen wir davon aus, dass ctrl.cfg.BaseDir der Projekt-Root ist, in dem auch der Ordner "uploads" liegt.
func (ctrl *controller) uploadsDir() string {
	return filepath.Join(ctrl.model.Config.Basedir, "uploads")
}

// Wandelt einen absoluten Pfad (unter uploadsDir) in eine öffentliche URL /uploads/<rel> um.
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
