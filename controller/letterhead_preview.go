// controller/letterhead_previews.go
package controller

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/billingcat/crm/model"
)

func (ctrl *controller) userAssetAbsToURL(ownerID uint, abs string) (string, error) {
	// Assumption: /userassets is mounted as a static route for <basedir>/assets/userassets.
	root := ctrl.userAssetsDir(ownerID)
	absRoot, _ := filepath.Abs(root)
	absFile, _ := filepath.Abs(abs)
	if !strings.HasPrefix(absFile, absRoot+string(os.PathSeparator)) && absFile != absRoot {
		return "", errors.New("file not under user asset root")
	}
	rel, err := filepath.Rel(root, absFile)
	if err != nil {
		return "", err
	}
	// Erzeuge öffentliche URL
	return "/userassets/" + fmt.Sprintf("owner%d/", ownerID) + filepath.ToSlash(rel), nil
}

// ensureLetterheadPreviews renders up to 2 PNGs and returns dimensions + public URLs.
func (ctrl *controller) ensureLetterheadPreviews(ownerID uint, tpl *model.LetterheadTemplate) (wcm, hcm float64, page1URL, page2URL string, err error) {
	userRoot := ctrl.userAssetsDir(ownerID)
	pdfAbs, err := safeJoin(userRoot, tpl.PDFPath)
	if err != nil {
		return 0, 0, "", "", err
	}

	// Previews are generated into the uploads directory.
	outDir := filepath.Join(
		ctrl.uploadsDir(),
		"letterhead",
		fmt.Sprintf("owner%d", ownerID),
		fmt.Sprintf("%d", tpl.ID),
	)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return 0, 0, "", "", err
	}

	sizes, pngs, err := renderPDFToPNGs(pdfAbs, outDir, 144, 2)
	if err != nil {
		return 0, 0, "", "", err
	}
	if len(pngs) == 0 {
		return 0, 0, "", "", errors.New("no preview generated")
	}

	url1, err := ctrl.uploadsAbsToURL(pngs[0])
	if err != nil {
		return 0, 0, "", "", err
	}
	var url2 string
	if len(pngs) > 1 {
		url2, err = ctrl.uploadsAbsToURL(pngs[1])
		if err != nil {
			return 0, 0, "", "", err
		}
	}

	wcm, hcm = round2(sizes[0][0]), round2(sizes[0][1])
	return wcm, hcm, url1, url2, nil
}
