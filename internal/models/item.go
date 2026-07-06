package models

import "github.com/google/uuid"

type Item struct {
	ID         uuid.UUID  `json:"id"`
	Name       string     `json:"name"`
	CategoryID uuid.UUID  `json:"categoryId"`
	IsSystem   bool       `json:"isSystem"`
	UserID     *uuid.UUID `json:"-"` // nil for system items
}
