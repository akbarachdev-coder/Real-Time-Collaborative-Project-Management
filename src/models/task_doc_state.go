package models

import "time"

type TaskDocState struct {
	TaskID       uint      `gorm:"primaryKey" json:"task_id"`
	CRDTSnapshot []byte    `gorm:"type:longblob" json:"-"`
	ClockVersion int       `gorm:"default:1" json:"clock_version"`
	UpdatedAt    time.Time `json:"updated_at"`
}
