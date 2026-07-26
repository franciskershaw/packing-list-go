package models

import "github.com/google/uuid"

type Template struct {
	ID          uuid.UUID      `json:"id"`
	Name        string         `json:"name"`
	Description *string        `json:"description"`
	Items       []TemplateItem `json:"items"`
	// ItemCount is authoritative from GetTemplates (a COUNT query);
	// GetTemplateByID sets it to len(Items) instead. See PACK-034.
	ItemCount int       `json:"itemCount"`
	UserID    uuid.UUID `json:"-"`
}

// TemplateItem represents an item attached to a template, with the quantity
// and notes for that template. Always empty until template-items endpoints
// exist to populate it.
type TemplateItem struct {
	ItemID   uuid.UUID `json:"itemId"`
	Name     string    `json:"name"`
	Quantity int       `json:"quantity"`
	Notes    *string   `json:"notes"`
}
