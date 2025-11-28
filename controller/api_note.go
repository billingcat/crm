package controller

import "time"

type APINote struct {
	ID         uint      `json:"id" xml:"id,attr"`
	CreatedAt  time.Time `json:"created_at" xml:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" xml:"updated_at"`
	AuthorID   uint      `json:"author_id" xml:"author_id"`
	ParentID   uint      `json:"parent_id" xml:"parent_id"`
	ParentType string    `json:"parent_type" xml:"parent_type"`
	Title      string    `json:"title" xml:"title"`
	Body       string    `json:"body" xml:"body"`
	Tags       string    `json:"tags" xml:"tags"`
	EditedAt   time.Time `json:"edited_at" xml:"edited_at"`
}
