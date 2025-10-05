package controller

import (
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"

	"github.com/gen2brain/go-fitz"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func parseUintParam(c echo.Context, name string) (uint, error) {
	val := c.Param(name)
	var id64 uint64
	_, err := fmt.Sscanf(val, "%d", &id64)
	return uint(id64), err
}

func renderPDFToPNGs(pdfPath, outDir string, dpi, maxPages int) (sizes [][2]float64, pngPaths []string, err error) {
	if dpi <= 0 {
		dpi = 200 // sinnvolles Fallback
	}

	doc, err := fitz.New(pdfPath)
	if err != nil {
		return nil, nil, err
	}
	defer doc.Close()

	num := doc.NumPage()
	if maxPages > 0 && num > maxPages {
		num = maxPages
	}
	if num == 0 {
		return nil, nil, errors.New("no pages")
	}

	for i := 0; i < num; i++ {
		// Wichtig: mit der gewünschten DPI rendern!
		img, err := doc.ImageDPI(i, float64(dpi))
		if err != nil {
			return nil, nil, err
		}

		fn := filepath.Join(outDir, fmt.Sprintf("pg_%02d_%s.png", i+1, uuid.New().String()))
		if err := savePNG(fn, img); err != nil {
			return nil, nil, err
		}

		b := img.Bounds()
		wpx, hpx := b.Dx(), b.Dy()

		// Pixel → cm mit derselben DPI
		wcm := float64(wpx) * 2.54 / float64(dpi)
		hcm := float64(hpx) * 2.54 / float64(dpi)

		sizes = append(sizes, [2]float64{wcm, hcm})
		pngPaths = append(pngPaths, fn)
	}
	return sizes, pngPaths, nil
}

func savePNG(path string, m image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, m)
}

func round2(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}
