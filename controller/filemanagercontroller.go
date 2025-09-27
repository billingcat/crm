package controller

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

func (ctrl *controller) fileManagerInit(e *echo.Echo) {
	g := e.Group("/filemanager")
	g.Use(ctrl.authMiddleware)
	g.GET("", ctrl.filemanagerList)
	g.POST("/upload", ctrl.filemanagerUploadHandler)
	g.POST("/delete", ctrl.filemanagerDeleteHandler)
	g.GET("/download/*", ctrl.filemanagerDownloadHandler) // z.B. /download/foo.txt

}

const maxQuota = 5 * 1024 * 1024 // 5 MB

type FileRow struct {
	Name      string
	Size      int64
	SizeHuman string
	ModTime   time.Time
	IsDir     bool
}

// calcDirSize summiert rekursiv die Dateigröße
func calcDirSize(path string) (int64, error) {
	var size int64
	err := filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			size += info.Size()
		}
		return nil
	})
	return size, err
}

func humanSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB",
		float64(bytes)/float64(div), "KMGTPE"[exp])
}

// safeJoin stellt sicher, dass path IM baseDir liegt (kein Path Traversal).
func safeJoin(base, name string) (string, error) {
	clean := filepath.Clean("/" + name) // führt z.B. "../" neutralisiert
	rel := strings.TrimPrefix(clean, "/")
	full := filepath.Join(base, rel)
	// Absicherung, dass full unter base liegt:
	baseAbs, _ := filepath.Abs(base)
	fullAbs, _ := filepath.Abs(full)
	if !strings.HasPrefix(fullAbs, baseAbs+string(os.PathSeparator)) && fullAbs != baseAbs {
		return "", echo.NewHTTPError(http.StatusBadRequest, "invalid path")
	}
	return full, nil
}
func (ctrl *controller) filemanagerList(c echo.Context) error {
	m := ctrl.defaultResponseMap(c, "Dateimanager")
	m["action"] = "/filemanager"
	m["submit"] = "Speichern"
	m["cancel"] = "/"
	ownerID := c.Get("ownerid")

	dirPath := filepath.Join(ctrl.model.Config.Basedir, "assets", "userassets", fmt.Sprintf("owner%d", ownerID))
	// Sicherstellen, dass der Ordner existiert
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	var rows []FileRow
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		rows = append(rows, FileRow{
			Name:      filepath.Join(e.Name()),
			Size:      info.Size(),
			SizeHuman: humanSize(info.Size()),
			ModTime:   info.ModTime(),
			IsDir:     e.IsDir(),
		})
	}

	m["Files"] = rows
	m["CurrDir"] = dirPath

	return c.Render(http.StatusOK, "filemanager.html", m)
}

func (ctrl *controller) filemanagerUploadHandler(c echo.Context) error {
	// Optional: Subdir
	baseDir := filepath.Join(ctrl.model.Config.Basedir, "assets", "userassets", fmt.Sprintf("owner%d", c.Get("ownerid")))

	// aktuelle Größe berechnen
	used, err := calcDirSize(baseDir)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	form, err := c.MultipartForm()
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid multipart form")
	}
	files := form.File["files"]

	// Größe der Uploads addieren
	var newSize int64
	for _, fh := range files {
		newSize += fh.Size
	}

	if used+newSize > maxQuota {
		return echo.NewHTTPError(http.StatusRequestEntityTooLarge,
			fmt.Sprintf("Quota überschritten: %.2f MB von %.2f MB belegt",
				float64(used)/1024/1024, float64(maxQuota)/1024/1024))
	}

	for _, fh := range files {
		// Dateiname härten
		filename := filepath.Base(fh.Filename)

		dstPath, err := safeJoin(baseDir, filename)
		if err != nil {
			return err
		}

		src, err := fh.Open()
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		defer src.Close()

		// Datei erstellen (fehlschlagen, wenn existiert – optional)
		dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}

		if _, err := io.Copy(dst, src); err != nil {
			dst.Close()
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		dst.Close()
	}

	return c.Redirect(http.StatusSeeOther, "/filemanager")
}

func (ctrl *controller) filemanagerDeleteHandler(c echo.Context) error {
	path := c.FormValue("path") // relativer Pfad vom UI
	baseDir := filepath.Join(ctrl.model.Config.Basedir, "assets", "userassets", fmt.Sprintf("owner%d", c.Get("ownerid")))

	full, err := safeJoin(baseDir, path)
	if err != nil {
		return err
	}
	info, err := os.Stat(full)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}
	if info.IsDir() {
		// Optional: ganze Ordner löschen erlauben?
		return echo.NewHTTPError(http.StatusBadRequest, "refusing to delete directories")
	}
	if err := os.Remove(full); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.Redirect(http.StatusSeeOther, "/filemanager")
}

func (ctrl *controller) filemanagerDownloadHandler(c echo.Context) error {
	rel := strings.TrimPrefix(c.Param("*"), "/")
	baseDir := filepath.Join(ctrl.model.Config.Basedir, "assets", "userassets", fmt.Sprintf("owner%d", c.Get("ownerid")))

	full, err := safeJoin(baseDir, rel)
	if err != nil {
		return err
	}
	if _, err := os.Stat(full); err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}
	return c.Attachment(full, filepath.Base(full))
}
