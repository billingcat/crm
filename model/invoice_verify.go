package model

import (
	"strings"
)

func (crmdb *CRMDatenbank) VerifyInvoice(inv *Invoice, settings *Settings) []InvoiceProblem {
	var problems []InvoiceProblem
	isIntraCommunity := inv.TaxType == "K"
	isReverseCharge := inv.TaxType == "AE"
	if (isIntraCommunity || isReverseCharge) && inv.ExemptionReason == "" {
		problems = append(problems, InvoiceProblem{
			Level:   "error",
			Message: "Es muss ein Befreiungsgrund angegeben werden für eine innergemeinschaftliche Lieferung bzw. eine Rechnung mit Steuerschuldumkehr.",
		})
	}
	if isIntraCommunity {
		if inv.Company.VATID == "" {
			problems = append(problems, InvoiceProblem{
				Level:   "error",
				Message: "Es muss eine USt-IdNr. des Kunden angegeben werden für eine innergemeinschaftliche Lieferung.",
			})
		}
		if settings.VATID == "" {
			problems = append(problems, InvoiceProblem{
				Level:   "error",
				Message: "Es ist keine USt-IdNr. des eigenen Unternehmens hinterlegt. Diese wird für eine innergemeinschaftliche Lieferung benötigt.",
			})
		}
		if inv.Company.Land == settings.CountryCode {
			problems = append(problems, InvoiceProblem{
				Level:   "error",
				Message: "Das Land des Kunden ist identisch mit dem eigenen Land. Eine innergemeinschaftliche Lieferung ist nur an Unternehmen in anderen EU-Ländern möglich.",
			})
		}
		if inv.Company.Land == "" {
			problems = append(problems, InvoiceProblem{
				Level:   "error",
				Message: "Es muss ein Land des Kunden angegeben werden für eine innergemeinschaftliche Lieferung.",
			})
		}
	}

	// [BR-25]-Each Invoice line (BG-25) shall contain the Item name (BT-153).
	hasEmptyInvoicePositionText := false
	for _, pos := range inv.InvoicePositions {
		if strings.TrimSpace(pos.Text) == "" {
			hasEmptyInvoicePositionText = true
			break
		}
	}
	if hasEmptyInvoicePositionText {
		problems = append(problems, InvoiceProblem{
			Level:   "warning",
			Message: "Es gibt eine oder mehrere Positionen ohne Text. Jede Position soll einen Text haben.",
		})
	}
	// [BR-CO-26]-In order for the buyer to automatically identify a supplier, the Seller identifier (BT-29), the Seller legal registration identifier (BT-30) and/or the Seller VAT identifier (BT-31) shall be present.
	if settings.CompanyName == "" {
		problems = append(problems, InvoiceProblem{
			Level:   "error",
			Message: "Es ist kein Firmenname des eigenen Unternehmens hinterlegt.",
		})
	}
	if settings.Address1 == "" && settings.Address2 == "" {
		problems = append(problems, InvoiceProblem{
			Level:   "error",
			Message: "Es ist keine Adresse des eigenen Unternehmens hinterlegt.",
		})
	}
	if settings.City == "" {
		problems = append(problems, InvoiceProblem{
			Level:   "error",
			Message: "Es ist kein Ort des eigenen Unternehmens hinterlegt.",
		})
	}
	if settings.ZIP == "" {
		problems = append(problems, InvoiceProblem{
			Level:   "error",
			Message: "Es ist keine Postleitzahl des eigenen Unternehmens hinterlegt.",
		})
	}
	if settings.CountryCode == "" {
		problems = append(problems, InvoiceProblem{
			Level:   "error",
			Message: "Es ist kein Land des eigenen Unternehmens hinterlegt.",
		})
	}

	return problems
}
