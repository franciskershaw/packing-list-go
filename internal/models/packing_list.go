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
// archived_at exist in the schema but aren't exposed yet — PACK-012 owns
// surfacing sort_order; archived_at isn't exposed at all yet (no concrete
// consumer — see PACK-011 handoff doc).
type PackingListItem struct {
	ItemID     uuid.UUID `json:"itemId"`
	Name       string    `json:"name"`
	CategoryID uuid.UUID `json:"categoryId"`
	Quantity   int       `json:"quantity"`
	Notes      *string   `json:"notes"`
	IsPacked   bool      `json:"isPacked"`
	SortOrder  *int      `json:"sortOrder"`
}

// PackingListDetail is the GET /lists/:id (and PATCH /lists/:id) response
// shape — items grouped by category, unlike PackingList's flat Items.
type PackingListDetail struct {
	ID         uuid.UUID             `json:"id"`
	Name       string                `json:"name"`
	EventDate  *string               `json:"eventDate"`
	TemplateID *uuid.UUID            `json:"templateId"`
	Categories []PackingListCategory `json:"categories"`
	UserID     uuid.UUID             `json:"-"`
}

// PackingListCategory groups a packing list's items under the category
// they belong to. Only categories with at least one item on the list
// appear — no empty categories are padded in.
type PackingListCategory struct {
	ID    uuid.UUID               `json:"id"`
	Name  string                  `json:"name"`
	Items []PackingListDetailItem `json:"items"`
}

// PackingListDetailItem is an item as it appears nested under a
// PackingListCategory. No categoryId — it would just repeat the parent
// category's own id back to the client.
type PackingListDetailItem struct {
	ItemID    uuid.UUID `json:"itemId"`
	Name      string    `json:"name"`
	Quantity  int       `json:"quantity"`
	Notes     *string   `json:"notes"`
	IsPacked  bool      `json:"isPacked"`
	SortOrder *int      `json:"sortOrder"`
}
