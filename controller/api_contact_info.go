package controller

import "time"

type APIContactInfo struct {
	ID        uint      `json:"id" xml:"id,attr"`
	CreatedAt time.Time `json:"created_at" xml:"created_at"`
	UpdatedAt time.Time `json:"updated_at" xml:"updated_at"`
	Type      string    `json:"type" xml:"type"`
	Label     string    `json:"label" xml:"label"`
	Value     string    `json:"value" xml:"value"`
}
