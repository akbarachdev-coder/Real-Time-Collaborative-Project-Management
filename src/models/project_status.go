package models

import "time"

type ProjectStatus struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ProjectID uint      `gorm:"index;not null" json:"project_id"`
	Name      string    `gorm:"size:100;not null" json:"name"`
	Color     string    `gorm:"size:7;default:#888888" json:"color"`
	Position  float64   `gorm:"not null" json:"position"`
	CreatedAt time.Time `json:"created_at"`
}
