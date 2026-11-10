package models

import "time"

type Project struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:150;not null" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	OwnerID     uint      `gorm:"index;not null" json:"owner_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ProjectMember struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ProjectID uint      `gorm:"uniqueIndex:idx_project_member;not null" json:"project_id"`
	UserID    uint      `gorm:"uniqueIndex:idx_project_member;not null" json:"user_id"`
	Role      string    `gorm:"size:20;default:member" json:"role"`
	CreatedAt time.Time `json:"created_at"`
}
