//go:build !cgo
// +build !cgo

package controller

import (
	"fmt"
	"image"

	"github.com/labstack/echo/v4"
)

func parseUintParam(c echo.Context, name string) (uint, error) {
	return uint(0), fmt.Errorf("PDF rendering not supported (built without cgo/fitz)")
}

func renderPDFToPNGs(pdfPath, outDir string, dpi, maxPages int) (sizes [][2]float64, pngPaths []string, err error) {
	return nil, nil, fmt.Errorf("PDF rendering not supported (built without cgo/fitz)")
}

func savePNG(path string, m image.Image) error {
	return fmt.Errorf("PDF rendering not supported (built without cgo/fitz)")
}

func round2(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}
