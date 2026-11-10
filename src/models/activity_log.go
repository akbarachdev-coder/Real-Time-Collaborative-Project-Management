package models

import "time"

type ActivityLog struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	WorkspaceID uint      `gorm:"index;not null" json:"workspace_id"`
	ProjectID   *uint     `gorm:"index" json:"project_id"`
	TaskID      *uint     `gorm:"index" json:"task_id"`
	UserID      *uint     `gorm:"index" json:"user_id"`
	ActionType  string    `gorm:"size:100;not null" json:"action_type"`
	Payload     string    `gorm:"type:json;not null" json:"payload"`
	CreatedAt   time.Time `json:"created_at"`
}
