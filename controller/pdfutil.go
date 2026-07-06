//go:build !cgo
// +build !cgo

package controller

import (
	"fmt"
	"image"
)

func renderPDFToPNGs(pdfPath, outDir string, dpi, maxPages int) (sizes [][2]float64, pngPaths []string, err error) {
	return nil, nil, fmt.Errorf("PDF rendering not supported (built without cgo/fitz)")
}

func savePNG(path string, m image.Image) error {
	return fmt.Errorf("PDF rendering not supported (built without cgo/fitz)")
}

func round2(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}
