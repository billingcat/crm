package model

import "testing"

// TestResolvePDFEngine covers the full decision matrix: explicit choice ×
// layout.xml presence × publishing-server configuration. A layout.xml without
// a configured server must error instead of silently falling back to
// boxesandglue (see docs/pdf-engine-dispatch.md).
func TestResolvePDFEngine(t *testing.T) {
	testcases := []struct {
		name             string
		choice           PDFEngine
		hasLayoutXML     bool
		serverConfigured bool
		want             PDFEngine
		wantErr          bool
	}{
		{"auto no layout no server", PDFEngineAuto, false, false, PDFEngineBag, false},
		{"auto no layout with server", PDFEngineAuto, false, true, PDFEngineBag, false},
		{"auto layout with server", PDFEngineAuto, true, true, PDFEngineSpeedata, false},
		{"auto layout no server errors", PDFEngineAuto, true, false, "", true},
		{"empty choice behaves like auto", "", true, true, PDFEngineSpeedata, false},
		{"explicit bag ignores layout and server", PDFEngineBag, true, true, PDFEngineBag, false},
		{"explicit bag without anything", PDFEngineBag, false, false, PDFEngineBag, false},
		{"explicit speedata with server", PDFEngineSpeedata, false, true, PDFEngineSpeedata, false},
		{"explicit speedata no server errors", PDFEngineSpeedata, true, false, "", true},
		{"unknown choice errors", "wordperfect", false, true, "", true},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolvePDFEngine(tc.choice, tc.hasLayoutXML, tc.serverConfigured)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("resolvePDFEngine(%q, %v, %v): expected error, got %q",
						tc.choice, tc.hasLayoutXML, tc.serverConfigured, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolvePDFEngine(%q, %v, %v): unexpected error: %v",
					tc.choice, tc.hasLayoutXML, tc.serverConfigured, err)
			}
			if got != tc.want {
				t.Errorf("resolvePDFEngine(%q, %v, %v) = %q, want %q",
					tc.choice, tc.hasLayoutXML, tc.serverConfigured, got, tc.want)
			}
		})
	}
}
