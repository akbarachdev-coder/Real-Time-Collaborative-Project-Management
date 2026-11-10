package models

import "time"

type Comment struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TaskID    uint      `gorm:"index;not null" json:"task_id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	ParentID  *uint     `gorm:"index" json:"parent_id"`
	Content   string    `gorm:"type:json;not null" json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
