package models

import "time"

const (
	PriorityLow    = "LOW"
	PriorityMedium = "MEDIUM"
	PriorityHigh   = "HIGH"
	PriorityUrgent = "URGENT"
)

func IsValidPriority(p string) bool {
	switch p {
	case PriorityLow, PriorityMedium, PriorityHigh, PriorityUrgent:
		return true
	}
	return false
}

type Task struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	ProjectID   uint       `gorm:"index;not null" json:"project_id"`
	StatusID    uint       `gorm:"index;not null" json:"status_id"`
	TaskNumber  int        `gorm:"not null" json:"task_number"`
	Title       string     `gorm:"size:500;not null" json:"title"`
	Description string     `gorm:"type:json" json:"description"`
	Priority    string     `gorm:"size:10;default:MEDIUM" json:"priority"`
	Position    float64    `gorm:"not null" json:"position"`
	DueDate     *time.Time `json:"due_date"`
	ReporterID  *uint      `gorm:"index" json:"reporter_id"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type TaskAssignee struct {
	TaskID     uint      `gorm:"primaryKey" json:"task_id"`
	UserID     uint      `gorm:"primaryKey" json:"user_id"`
	AssignedAt time.Time `json:"assigned_at"`
}
