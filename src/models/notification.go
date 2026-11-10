package models

import "time"

const (
	NotificationTaskAssigned = "task_assigned"
	NotificationCommentAdded = "comment_added"
	NotificationMemberAdded  = "member_added"
	NotificationMemberRemoved = "member_removed"
)

type Notification struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	Type      string    `gorm:"size:50;not null" json:"type"`
	Message   string    `gorm:"size:500;not null" json:"message"`
	ProjectID *uint     `gorm:"index" json:"project_id"`
	TaskID    *uint     `gorm:"index" json:"task_id"`
	IsRead    bool      `gorm:"default:false;index" json:"is_read"`
	CreatedAt time.Time `json:"created_at"`
}
