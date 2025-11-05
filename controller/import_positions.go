package controller

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

// Response payload returned to the browser.
// Matches the client-side JSON schema used by your in-page importer.
type importResponse struct {
	Version   int              `json:"version"`
	Positions []map[string]any `json:"positions"`
}

// Public DTO used by all parsers
type ImportedPosition struct {
	Text     string   // required
	Quantity float64  // required
	NetPrice float64  // required
	TaxRate  *float64 // optional (nil => use company default)
	Unit     string   // optional ("" => "C62")
}

// Convenience: one entry point that auto-detects by file extension or content.
// - ext can be "", ".csv", ".xml" (case-insensitive). If empty, content sniffing is used.
func ParsePositions(r io.Reader, ext string) ([]ImportedPosition, error) {
	ext = strings.ToLower(ext)
	// Read all to allow sniffing + reuse
	all, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	trim := bytes.TrimSpace(all)

	// Decide by extension first, else content
	switch ext {
	case ".csv":
		return parseCSV(bytes.NewReader(all))
	case ".xml":
		return parseXML(bytes.NewReader(all))
	case "":
		// Sniff: XML if it starts with '<', else CSV
		if len(trim) > 0 && trim[0] == '<' {
			return parseXML(bytes.NewReader(all))
		}
		return parseCSV(bytes.NewReader(all))
	default:
		return nil, fmt.Errorf("unsupported extension: %s (use .csv or .xml)", ext)
	}
}

// CSV
// Expected header: text;quantity;net_price;tax_rate;unit
// - Separator can be ';' or ','
// - Decimal comma allowed (e.g., "3,5")
// - tax_rate optional
func parseCSV(r io.Reader) ([]ImportedPosition, error) {
	// Peek first non-empty line to detect separator
	br := bufio.NewReader(r)
	var headerLine string
	for {
		b, err := br.ReadString('\n')
		if err != nil && err != io.EOF {
			return nil, err
		}
		line := strings.TrimSpace(b)
		if line != "" {
			headerLine = line
		}
		// Rebuild stream: headerLine + rest
		rest, _ := io.ReadAll(br)
		full := headerLine + "\n" + string(rest)

		sep := ';'
		if strings.Count(headerLine, ";") == 0 && strings.Count(headerLine, ",") > 0 {
			sep = ','
		}
		cr := csv.NewReader(strings.NewReader(full))
		cr.Comma = sep
		cr.FieldsPerRecord = -1 // allow variable fields

		rows, err := cr.ReadAll()
		if err != nil {
			return nil, fmt.Errorf("csv parse error: %w", err)
		}
		if len(rows) < 2 {
			return nil, fmt.Errorf("csv has no data rows")
		}
		// header map
		header := make([]string, len(rows[0]))
		for i := range rows[0] {
			header[i] = strings.ToLower(strings.TrimSpace(rows[0][i]))
		}
		idx := func(name string) int {
			for i, h := range header {
				if h == name {
					return i
				}
			}
			return -1
		}
		textIdx := idx("text")
		qtyIdx := idx("quantity")
		priceIdx := idx("net_price")
		if textIdx < 0 || qtyIdx < 0 || priceIdx < 0 {
			return nil, fmt.Errorf("csv header must contain at least: text, quantity, net_price")
		}
		taxIdx := idx("tax_rate")
		unitIdx := idx("unit")

		var out []ImportedPosition
		for ri := 1; ri < len(rows); ri++ {
			rec := rows[ri]
			// Skip pure empty lines
			isEmpty := true
			for _, c := range rec {
				if strings.TrimSpace(c) != "" {
					isEmpty = false
					break
				}
			}
			if isEmpty {
				continue
			}

			get := func(i int) string {
				if i < 0 || i >= len(rec) {
					return ""
				}
				return strings.TrimSpace(rec[i])
			}

			text := get(textIdx)
			if text == "" {
				return nil, fmt.Errorf("row %d: text is required", ri+1)
			}

			qty, err := parseLocalizedFloat(get(qtyIdx))
			if err != nil {
				return nil, fmt.Errorf("row %d: invalid quantity: %v", ri+1, err)
			}
			price, err := parseLocalizedFloat(get(priceIdx))
			if err != nil {
				return nil, fmt.Errorf("row %d: invalid net_price: %v", ri+1, err)
			}

			var taxPtr *float64
			if taxIdx >= 0 {
				if s := get(taxIdx); s != "" {
					tax, err := parseLocalizedFloat(s)
					if err != nil {
						return nil, fmt.Errorf("row %d: invalid tax_rate: %v", ri+1, err)
					}
					taxPtr = &tax
				}
			}

			unit := strings.ToUpper(get(unitIdx))
			if unit == "" {
				unit = "C62"
			}

			out = append(out, ImportedPosition{
				Text:     text,
				Quantity: qty,
				NetPrice: price,
				TaxRate:  taxPtr,
				Unit:     unit,
			})
		}
		return out, nil
	}
}

// XML
// Format:
// <invoice version="1"><positions><position>...</position></positions></invoice>
type xmlInvoice struct {
	XMLName   xml.Name      `xml:"invoice"`
	Version   string        `xml:"version,attr"`
	Positions []xmlPosition `xml:"positions>position"`
}
type xmlPosition struct {
	Text     string  `xml:"text"`
	Quantity string  `xml:"quantity"`
	NetPrice string  `xml:"net_price"`
	TaxRate  *string `xml:"tax_rate"`
	Unit     string  `xml:"unit"`
}

func parseXML(r io.Reader) ([]ImportedPosition, error) {
	var inv xmlInvoice
	dec := xml.NewDecoder(r)
	dec.CharsetReader = charsetReader // handle utf-8/iso-8859-1 gracefully
	if err := dec.Decode(&inv); err != nil {
		return nil, fmt.Errorf("xml parse error: %w", err)
	}
	if len(inv.Positions) == 0 {
		return nil, fmt.Errorf("xml contains no positions")
	}

	out := make([]ImportedPosition, 0, len(inv.Positions))
	for i, p := range inv.Positions {
		text := strings.TrimSpace(p.Text)
		if text == "" {
			return nil, fmt.Errorf("position %d: text is required", i+1)
		}
		qty, err := parseLocalizedFloat(p.Quantity)
		if err != nil {
			return nil, fmt.Errorf("position %d: invalid quantity: %v", i+1, err)
		}
		price, err := parseLocalizedFloat(p.NetPrice)
		if err != nil {
			return nil, fmt.Errorf("position %d: invalid net_price: %v", i+1, err)
		}
		var taxPtr *float64
		if p.TaxRate != nil && strings.TrimSpace(*p.TaxRate) != "" {
			tax, err := parseLocalizedFloat(*p.TaxRate)
			if err != nil {
				return nil, fmt.Errorf("position %d: invalid tax_rate: %v", i+1, err)
			}
			taxPtr = &tax
		}
		unit := strings.ToUpper(strings.TrimSpace(p.Unit))
		if unit == "" {
			unit = "C62"
		}

		out = append(out, ImportedPosition{
			Text:     text,
			Quantity: qty,
			NetPrice: price,
			TaxRate:  taxPtr,
			Unit:     unit,
		})
	}
	return out, nil
}

//
// Helpers
//

// Accepts "3,5", "3.5", " 95.00 " etc.
func parseLocalizedFloat(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty number")
	}
	// Replace German decimal comma, keep only digits, minus, dot, comma
	s = cleanupNumberString(s)
	s = strings.ReplaceAll(s, ",", ".")
	return strconv.ParseFloat(s, 64)
}

// Remove thin spaces, NBSP, and normalize weird grouping
var nonDigitRe = regexp.MustCompile(`[^\d\-,\.]`)

func cleanupNumberString(s string) string {
	// remove any currency signs / spaces
	s = nonDigitRe.ReplaceAllString(s, "")
	// collapse multiple dots/commas sensibly is out of scope here; assume input is reasonable
	return s
}

// Basic charset handler — return nil to let stdlib handle utf-8; for other encodings you may add support.
func charsetReader(charset string, input io.Reader) (io.Reader, error) {
	// For UTF-8, return as-is. If you need ISO-8859-1 etc., plug in golang.org/x/net/html/charset.
	return input, nil
}

// importPositionsAPI accepts multipart/form-data with field "file".
// It parses CSV or XML via importpositions.ParsePositions and returns
// the normalized JSON structure ({version:1, positions:[...]}).
func (ctrl *controller) importPositionsAPI(c echo.Context) error {
	// Optional: Limit the size early (Echo also allows global BodyLimit middleware)
	// c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, 25<<20)

	// Parse form (Echo does this lazily, but FormFile needs it ready)
	if err := c.Request().ParseMultipartForm(25 << 20); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "multipart error: "+err.Error())
	}

	file, header, err := c.Request().FormFile("file")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "file missing: "+err.Error())
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))

	// Use shared parser (CSV/XML auto handling)
	imports, err := ParsePositions(file, ext)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "parse error: "+err.Error())
	}

	// Transform into the uniform JSON expected by your client
	resp := importResponse{
		Version:   1,
		Positions: make([]map[string]any, 0, len(imports)),
	}
	for _, p := range imports {
		row := map[string]any{
			"text":      p.Text,
			"quantity":  p.Quantity, // float64
			"net_price": p.NetPrice, // float64
			"unit":      strings.ToUpper(p.Unit),
		}
		if p.TaxRate != nil {
			row["tax_rate"] = *p.TaxRate // float64
		}
		resp.Positions = append(resp.Positions, row)
	}

	// Echo will JSON-encode for you with correct headers
	return c.JSON(http.StatusOK, resp)
}

// (Optional) If you also want to support JSON upload via the same endpoint,
// you could branch on Content-Type application/json and proxy through.
func (ctrl *controller) importPositionsAPIJSON(c echo.Context) error {
	var payload struct {
		Version   int `json:"version"`
		Positions []struct {
			Text     string   `json:"text"`
			Quantity float64  `json:"quantity"`
			NetPrice float64  `json:"net_price"`
			TaxRate  *float64 `json:"tax_rate"`
			Unit     string   `json:"unit"`
		} `json:"positions"`
	}
	if err := json.NewDecoder(c.Request().Body).Decode(&payload); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid json: "+err.Error())
	}
	// simply echo back the payload (already in the correct shape)
	return c.JSON(http.StatusOK, payload)
}
