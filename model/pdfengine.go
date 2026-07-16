package model

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// PDFEngine selects which engine renders invoice PDFs. Both engines are always
// compiled in; the choice is made at runtime per owner (settings field
// pdf_engine), see docs/pdf-engine-dispatch.md.
type PDFEngine string

const (
	// PDFEngineAuto picks speedata when the owner has a layout.xml, otherwise
	// boxesandglue. This is the default.
	PDFEngineAuto PDFEngine = "auto"
	// PDFEngineSpeedata renders on the remote speedata Publishing Server.
	PDFEngineSpeedata PDFEngine = "speedata"
	// PDFEngineBag renders locally in-process with boxesandglue/bagme.
	PDFEngineBag PDFEngine = "boxesandglue"
)

// resolvePDFEngine decides which engine renders the PDF. It is pure so the
// full decision matrix (choice × layout.xml × server config) is testable.
//
// A layout.xml only makes sense with speedata, so "auto" without a configured
// publishing server is an error rather than a silent fallback to boxesandglue:
// whoever maintains a layout.xml must not unknowingly get a different layout
// on their invoices.
func resolvePDFEngine(choice PDFEngine, hasLayoutXML, serverConfigured bool) (PDFEngine, error) {
	switch choice {
	case PDFEngineBag:
		return PDFEngineBag, nil
	case PDFEngineSpeedata:
		if !serverConfigured {
			return "", fmt.Errorf("PDF-Engine speedata gewählt, aber kein Publishing-Server konfiguriert")
		}
		return PDFEngineSpeedata, nil
	case PDFEngineAuto, "":
		if !hasLayoutXML {
			return PDFEngineBag, nil
		}
		if !serverConfigured {
			return "", fmt.Errorf("layout.xml vorhanden, aber kein Publishing-Server konfiguriert")
		}
		return PDFEngineSpeedata, nil
	default:
		return "", fmt.Errorf("unbekannte PDF-Engine %q", choice)
	}
}

// hasUserLayoutXML reports whether the owner keeps a layout.xml in their
// userassets directory (the speedata layout file).
func (s *Store) hasUserLayoutXML(ownerID uint) bool {
	p := filepath.Join(s.Config.Basedir, "assets", "userassets", fmt.Sprintf("owner%d", ownerID), "layout.xml")
	_, err := os.Stat(p)
	return err == nil
}

// speedataConfigured reports whether config.toml points to a publishing server.
func (s *Store) speedataConfigured() bool {
	return s.Config.PublishingServerAddress != ""
}

// ResolvePDFEngine determines the effective engine for the owner from their
// settings, their userassets and the server configuration.
func (s *Store) ResolvePDFEngine(ownerID uint) (PDFEngine, error) {
	settings, err := s.LoadSettings(ownerID)
	if err != nil {
		return "", fmt.Errorf("load settings: %w", err)
	}
	return resolvePDFEngine(PDFEngine(settings.PDFEngine), s.hasUserLayoutXML(ownerID), s.speedataConfigured())
}

// CreateZUGFeRDPDF creates a ZUGFeRD PDF file for the invoice with the engine
// resolved for the owner. The CII XML is expected to exist at xmlpath and the
// PDF gets written to pdfpath.
func (s *Store) CreateZUGFeRDPDF(inv *Invoice, ownerID uint, xmlpath string, pdfpath string, logger *slog.Logger) error {
	engine, err := s.ResolvePDFEngine(ownerID)
	if err != nil {
		return err
	}
	if engine == PDFEngineSpeedata {
		return s.createZUGFeRDPDFSpeedata(inv, ownerID, xmlpath, pdfpath, logger)
	}
	return s.createZUGFeRDPDFBag(inv, ownerID, xmlpath, pdfpath, logger)
}

// AutoLayoutNote describes, for the UI, what the "Automatisch" letterhead
// choice renders for this owner with the engine resolved from their settings.
func (s *Store) AutoLayoutNote(ownerID uint) string {
	engine, err := s.ResolvePDFEngine(ownerID)
	if err != nil {
		return "Achtung: " + err.Error() + " — die PDF-Erzeugung schlägt fehl."
	}
	if engine == PDFEngineSpeedata {
		return `Verwendet "layout.xml" über den speedata Publishing-Server.`
	}
	return "Verwendet das eingebaute Standard-Layout (DIN 5008) mit den Firmendaten aus den Einstellungen."
}
