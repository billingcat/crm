package model

import (
	"bytes"
	"encoding/xml"
	"os"
	"time"
)

// Settings is the root node written to settings.xml
type LetterheadSettings struct {
	XMLName       xml.Name        `xml:"BillingcatSettings"`
	XMLNS         string          `xml:"xmlns,attr"`
	Version       string          `xml:"version,attr"` // schema version
	GeneratedAt   time.Time       `xml:"generatedAt"`  // UTC timestamp
	InvoiceID     uint            `xml:"invoiceId"`
	InvoiceNumber string          `xml:"invoiceNumber"`
	Units         string          `xml:"units"` // "cm"
	Letterhead    LetterheadBlock `xml:"letterhead"`
}

type FontsBlock struct {
	Normal string `xml:"normal,omitempty"`
	Bold   string `xml:"bold,omitempty"`
	Italic string `xml:"italic,omitempty"`
}

// LetterheadBlock captures template meta and its regions.
type LetterheadBlock struct {
	Name         string         `xml:"name"`
	PageWidthCm  float64        `xml:"pageWidthCm"`
	PageHeightCm float64        `xml:"pageHeightCm"`
	Regions      []RegionExport `xml:"regions>region"`
	PDFPath      string         `xml:"pdfPath,omitempty"`
	Fonts        FontsBlock     `xml:"fonts"`
}

// RegionExport represents one placed region; all measurements in cm.
type RegionExport struct {
	Kind        string  `xml:"kind,attr"` // e.g. "addressee","invoice","main_area"
	Page        int     `xml:"page,attr"` // primary rect page, 1-based
	XCm         float64 `xml:"xCm"`
	YCm         float64 `xml:"yCm"`
	WidthCm     float64 `xml:"widthCm"`
	HeightCm    float64 `xml:"heightCm"`
	HAlign      string  `xml:"hAlign,omitempty"`
	VAlign      string  `xml:"vAlign,omitempty"`
	FontName    string  `xml:"fontName,omitempty"`
	FontSizePt  float64 `xml:"fontSizePt,omitempty"`
	LineSpacing float64 `xml:"lineSpacing,omitempty"`

	// Optional second-page rectangle for "main_area" regions
	HasPage2  bool    `xml:"hasPage2,omitempty"`
	X2Cm      float64 `xml:"x2Cm,omitempty"`
	Y2Cm      float64 `xml:"y2Cm,omitempty"`
	Width2Cm  float64 `xml:"width2Cm,omitempty"`
	Height2Cm float64 `xml:"height2Cm,omitempty"`
}

// WriteSettingsXML writes the given LetterheadSettings structure as formatted XML
// to the specified output file (outFilename) and also returns the XML content as []byte.
// The file will be created or overwritten if it already exists.
func WriteSettingsXML(outFilename string, s LetterheadSettings) ([]byte, error) {
	var buf bytes.Buffer

	// Write XML header
	buf.WriteString(xml.Header)

	// Encode struct into buffer
	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")
	if err := enc.Encode(s); err != nil {
		return nil, err
	}
	if err := enc.Flush(); err != nil {
		return nil, err
	}

	// Write buffer content to file
	if err := os.WriteFile(outFilename, buf.Bytes(), 0644); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// buildSettingsFromInvoice converts Invoice+Template(+Regions) into an export.Settings struct.
// Assumes Template and Template.Regions are preloaded if present.
func buildSettingsFromInvoice(inv *Invoice) *LetterheadSettings {
	if inv.Template == nil {
		return nil
	}
	tpl := inv.Template
	xmlRegs := make([]RegionExport, 0, len(tpl.Regions))
	for _, r := range tpl.Regions {
		xmlRegs = append(xmlRegs, RegionExport{
			Kind:        string(r.Kind),
			Page:        max(1, r.Page),
			XCm:         r.XCm,
			YCm:         r.YCm,
			WidthCm:     r.WidthCm,
			HeightCm:    r.HeightCm,
			HAlign:      r.HAlign,
			VAlign:      r.VAlign,
			FontName:    r.FontName,
			FontSizePt:  r.FontSizePt,
			LineSpacing: r.LineSpacing,
			HasPage2:    r.HasPage2,
			X2Cm:        r.X2Cm,
			Y2Cm:        r.Y2Cm,
			Width2Cm:    r.Width2Cm,
			Height2Cm:   r.Height2Cm,
		})
	}

	s := &LetterheadSettings{
		XMLNS:         "urn:billingcat.de/ns/billingcatsettings",
		Version:       "1.0",
		GeneratedAt:   time.Now().UTC(),
		InvoiceID:     inv.ID,
		InvoiceNumber: inv.Number,
		Units:         "cm",
		Letterhead: LetterheadBlock{
			Name:         tpl.Name,
			PageWidthCm:  tpl.PageWidthCm,
			PageHeightCm: tpl.PageHeightCm,
			PDFPath:      tpl.PDFPath,
			Regions:      xmlRegs,
			Fonts: FontsBlock{
				Normal: tpl.FontNormal,
				Bold:   tpl.FontBold,
				Italic: tpl.FontItalic,
			},
		},
	}
	return s
}

func max(a, b int) int {
	if a >= b {
		return a
	}
	return b
}
