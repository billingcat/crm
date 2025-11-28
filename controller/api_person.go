package controller

import (
	"encoding/xml"
	"time"
)

type APIPerson struct {
	ID           uint             `json:"id" xml:"id,attr"`
	Name         string           `json:"name" xml:"name"`
	Position     string           `json:"position,omitempty" xml:"position,omitempty"`
	Email        string           `json:"email,omitempty" xml:"email,omitempty"`
	CompanyID    int              `json:"company_id,omitempty" xml:"company_id,omitempty"`
	ContactInfos []APIContactInfo `json:"contact_infos,omitempty" xml:"contact_infos>contact_info,omitempty"`
	Notes        []APINote        `json:"notes,omitempty" xml:"notes>note,omitempty"`
	CreatedAt    time.Time        `json:"created_at" xml:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at" xml:"updated_at"`
}

// Root-Element für persons.xml
type ExportPersons struct {
	XMLName xml.Name    `xml:"persons"`
	Version string      `xml:"version,attr,omitempty"`
	Persons []APIPerson `xml:"person"`
}
