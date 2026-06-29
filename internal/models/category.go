package models

import "github.com/google/uuid"

type Category struct {
	ID       uuid.UUID  `json:"id"`
	Name     string     `json:"name"`
	IsSystem bool       `json:"isSystem"`
	UserID   *uuid.UUID `json:"-"`
}
