package controller

import (
	"encoding/xml"
	"time"
)

// Letterhead templates + regions

type APILetterheadTemplate struct {
	ID              uint    `json:"id" xml:"id,attr"`
	Name            string  `json:"name" xml:"name"`
	PageWidthCm     float64 `json:"page_width_cm" xml:"page_width_cm"`
	PageHeightCm    float64 `json:"page_height_cm" xml:"page_height_cm"`
	PDFPath         string  `json:"pdf_path,omitempty" xml:"pdf_path,omitempty"`
	PreviewPage1URL string  `json:"preview_page1_url,omitempty" xml:"preview_page1_url,omitempty"`
	PreviewPage2URL string  `json:"preview_page2_url,omitempty" xml:"preview_page2_url,omitempty"`

	FontNormal string `json:"font_normal,omitempty" xml:"font_normal,omitempty"`
	FontBold   string `json:"font_bold,omitempty" xml:"font_bold,omitempty"`
	FontItalic string `json:"font_italic,omitempty" xml:"font_italic,omitempty"`

	Regions []APILetterheadRegion `json:"regions,omitempty" xml:"regions>region,omitempty"`

	CreatedAt time.Time `json:"created_at" xml:"created_at"`
	UpdatedAt time.Time `json:"updated_at" xml:"updated_at"`
}

type APILetterheadRegion struct {
	ID       uint    `json:"id" xml:"id,attr"`
	Kind     string  `json:"kind" xml:"kind"`
	Page     int     `json:"page" xml:"page"`
	XCm      float64 `json:"x_cm" xml:"x_cm"`
	YCm      float64 `json:"y_cm" xml:"y_cm"`
	WidthCm  float64 `json:"width_cm" xml:"width_cm"`
	HeightCm float64 `json:"height_cm" xml:"height_cm"`

	HAlign      string  `json:"h_align,omitempty" xml:"h_align,omitempty"`
	VAlign      string  `json:"v_align,omitempty" xml:"v_align,omitempty"`
	FontName    string  `json:"font_name,omitempty" xml:"font_name,omitempty"`
	FontSizePt  float64 `json:"font_size_pt,omitempty" xml:"font_size_pt,omitempty"`
	LineSpacing float64 `json:"line_spacing,omitempty" xml:"line_spacing,omitempty"`

	HasPage2  bool    `json:"has_page2,omitempty" xml:"has_page2,omitempty"`
	X2Cm      float64 `json:"x2_cm,omitempty" xml:"x2_cm,omitempty"`
	Y2Cm      float64 `json:"y2_cm,omitempty" xml:"y2_cm,omitempty"`
	Width2Cm  float64 `json:"width2_cm,omitempty" xml:"width2_cm,omitempty"`
	Height2Cm float64 `json:"height2_cm,omitempty" xml:"height2_cm,omitempty"`

	CreatedAt time.Time `json:"created_at" xml:"created_at"`
	UpdatedAt time.Time `json:"updated_at" xml:"updated_at"`
}

// Root-Element for XML export/import of letterhead templates
type ExportLetterheadTemplates struct {
	XMLName   xml.Name                `xml:"letterhead_templates"`
	Version   string                  `xml:"version,attr,omitempty"`
	Templates []APILetterheadTemplate `xml:"template"`
}
