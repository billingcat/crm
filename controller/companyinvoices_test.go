package controller

import (
	"strings"
	"testing"

	"github.com/billingcat/crm/model"
)

func TestSummarizePositions(t *testing.T) {
	positions := []model.InvoicePosition{
		{Text: "\n  Beratung\nzweite Zeile"},
		{Text: strings.Repeat("x", 100)},
		{Text: "Reisekosten"},
		{Text: "Spesen"},
		{Text: "Material"},
	}

	got, more := summarizePositions(positions)
	if len(got) != overviewMaxPositions {
		t.Fatalf("len = %d, want %d", len(got), overviewMaxPositions)
	}
	if more != 2 {
		t.Errorf("more = %d, want 2", more)
	}
	if got[0].Text != "Beratung" {
		t.Errorf("first text = %q, want %q", got[0].Text, "Beratung")
	}
	if n := len([]rune(got[1].Text)); n != overviewMaxTextRunes {
		t.Errorf("shortened text has %d runes, want %d", n, overviewMaxTextRunes)
	}

	got, more = summarizePositions(positions[:1])
	if len(got) != 1 || more != 0 {
		t.Errorf("single position: len = %d, more = %d", len(got), more)
	}
}
