package model

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	api "github.com/speedata/publisher-api"
)

func attachFile(p *api.PublishRequest, filename string, destFilename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	pf := api.PublishFile{Filename: destFilename, Contents: data}
	p.Files = append(p.Files, pf)
	return nil
}

func ensureDir(dirName string) error {
	err := os.MkdirAll(dirName, 0755)
	if err != nil {
		return err
	}
	return nil
}

// CreateZUGFeRDPDF creates a ZUGFeRD PDF file for the invoice. The XML is
// expected to exist at the given location and the PDF gets written to the
// location given by the last argument.
func (crmdb *CRMDatabase) CreateZUGFeRDPDF(inv *Invoice, ownerID uint, xmlpath string, pdfpath string, logger *slog.Logger) error {
	var err error
	var settingsData []byte
	if s := buildSettingsFromInvoice(inv); s != nil {
		settingsPath := filepath.Join(filepath.Dir(xmlpath), fmt.Sprintf("%d-settings.xml", inv.ID))
		if settingsData, err = WriteSettingsXML(settingsPath, *s); err != nil {
			// Do not fail invoice PDF creation just because settings.xml failed.
			logger.Error("write settings.xml failed", "err", err, "invoice_id", inv.ID)
		}
	}
	useTemplate := inv.TemplateID != nil && inv.Template != nil
	ep, err := api.NewEndpoint(crmdb.Config.PublishingServerUsername, crmdb.Config.PublishingServerAddress)
	if err != nil {
		return err
	}
	p := ep.NewPublishRequest()
	if settingsData != nil {
		p.Files = append(p.Files, api.PublishFile{
			Filename: "settings.xml",
			Contents: settingsData,
		})
	}
	if err = attachFile(p, xmlpath, "data.xml"); err != nil {
		return err
	}

	p.Version = "5.1.28"

	userAssetsDir := filepath.Join(crmdb.Config.Basedir, "assets", "userassets", fmt.Sprintf("owner%d", ownerID))

	if err = ensureDir(userAssetsDir); err != nil {
		return err
	}

	files, err := os.ReadDir(userAssetsDir)
	if err != nil {
		return err
	}
	hasLayout := false
	reject := map[string]bool{
		".DS_Store":     true,
		"publisher.cfg": true,
	}
	if useTemplate {
		reject["layout.xml"] = true // do not attach user layout if we have a custom template
	}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		dstFilename := file.Name()
		if reject[file.Name()] {
			continue
		}
		if file.Name() == "layout.xml" {
			hasLayout = true
		}
		fullPath := filepath.Join(userAssetsDir, file.Name())
		logger.Debug("attaching user asset", "file", fullPath)
		attachFile(p, fullPath, dstFilename)
	}

	// if has layout or has custom letterhead, we do not attach the generic layout
	if !hasLayout {
		// attach default layout
		genericLayout := filepath.Join(crmdb.Config.Basedir, "assets", "generic", "layout.xml")
		if err = attachFile(p, genericLayout, "layout.xml"); err != nil {
			return err
		}
	}
	resp, err := ep.Publish(p)
	if err != nil {
		return err
	}

	ps, err := resp.Wait()
	if err != nil {
		return err
	}

	if ps.Errors > 0 {
		logger.Error("PDF generation done", "errors", ps.Errors, "finishedAt", ps.Finished.Format(time.Stamp))
	} else {
		logger.Debug("PDF generation done", "errors", ps.Errors, "finishedAt", ps.Finished.Format(time.Stamp))
	}
	for _, e := range ps.Errormessages {
		logger.Error("error during PDF generation", "message", e.Error)
	}
	f, err := os.Create(pdfpath)
	if err != nil {
		return err
	}
	defer f.Close()
	err = resp.GetPDF(f)
	if err != nil {
		return err
	}
	return nil
}
