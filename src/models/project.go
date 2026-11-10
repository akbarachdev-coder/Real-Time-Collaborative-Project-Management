package models

import "time"

type Project struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID uint      `gorm:"index;not null" json:"workspace_id"`
	Name        string    `gorm:"size:255;not null" json:"name"`
	Identifier  string    `gorm:"size:10;not null" json:"identifier"`
	Description *string   `gorm:"type:text" json:"description"`
	IsPrivate   bool      `gorm:"default:false" json:"is_private"`
	CreatedBy   *uint     `gorm:"index" json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}
