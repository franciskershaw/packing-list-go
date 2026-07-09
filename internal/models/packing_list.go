package models

import "github.com/google/uuid"

type PackingList struct {
	ID         uuid.UUID         `json:"id"`
	Name       string            `json:"name"`
	EventDate  *string           `json:"eventDate"`
	TemplateID *uuid.UUID        `json:"templateId"`
	Items      []PackingListItem `json:"items"`
	UserID     uuid.UUID         `json:"-"`
}

// PackingListItem represents an item on a packing list, copied from a
// template's items (if any) at list-creation time. sort_order and
// archived_at exist in the schema but aren't exposed yet — PACK-012 and
// PACK-011 respectively are responsible for surfacing them.
type PackingListItem struct {
	ItemID     uuid.UUID `json:"itemId"`
	Name       string    `json:"name"`
	CategoryID uuid.UUID `json:"categoryId"`
	Quantity   int       `json:"quantity"`
	Notes      *string   `json:"notes"`
	IsPacked   bool      `json:"isPacked"`
}
