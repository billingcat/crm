// invoicecontroller_test.go
package controller

import (
	"fmt"
	"testing"
	"time"
)

func TestFormatInvoiceNumber(t *testing.T) {
	now := time.Now()
	year := now.Year()
	yy := fmt.Sprintf("%02d", year%100)
	yyyy := fmt.Sprintf("%04d", year)

	tests := []struct {
		name    string
		in      string
		cn      string
		counter int
		want    string
	}{
		{
			name:    "YYYY + CN + zero-padded counter (width 4)",
			in:      "RE-%YYYY%-%CN%-%04C%",
			cn:      "12345",
			counter: 7,
			want:    fmt.Sprintf("RE-%s-12345-0007", yyyy),
		},
		{
			name:    "YY + CN + non-padded counter (width given but no leading zero flag)",
			in:      "R-%YY%-%CN%-%3C%",
			cn:      "999",
			counter: 42,
			want:    fmt.Sprintf("R-%s-999-42", yy),
		},
		{
			name:    "Only year and CN, no counter",
			in:      "INV-%YYYY%-%CN%",
			cn:      "ACME",
			counter: 1,
			want:    fmt.Sprintf("INV-%s-ACME", yyyy),
		},
		{
			name:    "Multiple counter placeholders are replaced (same value/format)",
			in:      "X-%02C%-%02C%",
			cn:      "IG",
			counter: 3,
			want:    "X-03-03",
		},
		{
			name:    "Empty customer number stays empty",
			in:      "INV-%YYYY%-%CN%-%02C%",
			cn:      "",
			counter: 3,
			want:    fmt.Sprintf("INV-%s--03", yyyy),
		},
		{
			name:    "Large padding width",
			in:      "%YYYY%-%06C%",
			cn:      "X",
			counter: 1234,
			want:    fmt.Sprintf("%s-001234", yyyy),
		},
		{
			name:    "YY and YYYY used at the same time",
			in:      "Y%YY%/%YYYY%-%CN%-%02C%",
			cn:      "CNO",
			counter: 9,
			want:    fmt.Sprintf("Y%s/%s-CNO-09", yy, yyyy),
		},
		{
			name:    "No known placeholders",
			in:      "PLAIN",
			cn:      "ANY",
			counter: 99,
			want:    "PLAIN",
		},
		{
			name:    "CN without year, with non-padded counter",
			in:      "%CN%-%1C%",
			cn:      "KND",
			counter: 5,
			want:    "KND-5",
		},

		// ---- NEW: %C% support (no width) ----
		{
			name:    "Plain %C% without width",
			in:      "INV-%C%",
			cn:      "X",
			counter: 42,
			want:    "INV-42",
		},
		{
			name:    "Multiple %C% occurrences",
			in:      "A-%C%-B-%C%",
			cn:      "X",
			counter: 7,
			want:    "A-7-B-7",
		},
		{
			name:    "Edge: %0C% (zero flag without width) behaves like %C%",
			in:      "EDGE-%0C%",
			cn:      "X",
			counter: 5,
			want:    "EDGE-5",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := formatInvoiceNumber(tc.in, tc.cn, tc.counter)
			if got != tc.want {
				t.Fatalf("formatInvoiceNumber(%q, %q, %d) = %q, want %q",
					tc.in, tc.cn, tc.counter, got, tc.want)
			}
		})
	}
}

// Benchmark to measure performance of the replacer function
func BenchmarkFormatInvoiceNumber(b *testing.B) {
	in := "RE-%YYYY%-%CN%-%06C%"
	cn := "4711"
	for i := 0; i < b.N; i++ {
		_ = formatInvoiceNumber(in, cn, 123)
	}
}
