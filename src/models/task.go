package models

import "time"

const (
	TaskStatusTodo        = "todo"
	TaskStatusInProgress  = "in_progress"
	TaskStatusReview      = "review"
	TaskStatusDone        = "done"
)

type Task struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	ProjectID   uint       `gorm:"index;not null" json:"project_id"`
	Title       string     `gorm:"size:200;not null" json:"title"`
	Description string     `gorm:"type:text" json:"description"`
	Status      string     `gorm:"size:20;default:todo;index" json:"status"`
	Position    float64    `gorm:"default:0" json:"position"`
	AssigneeID  *uint      `gorm:"index" json:"assignee_id"`
	DueDate     *time.Time `json:"due_date"`
	CreatedBy   uint       `gorm:"index;not null" json:"created_by"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
