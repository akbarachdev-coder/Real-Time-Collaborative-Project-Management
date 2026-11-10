package services

import (
	"encoding/json"
	"log"

	"collabpm/models"

	"gorm.io/gorm"
)

// LogActivity writes an audit row. Failures are logged, never returned —
// activity must not break the primary operation.
func LogActivity(db *gorm.DB, workspaceID uint, projectID, taskID, userID *uint, actionType string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("activity: marshal failed: %v", err)
		return
	}
	entry := models.ActivityLog{
		WorkspaceID: workspaceID,
		ProjectID:   projectID,
		TaskID:      taskID,
		UserID:      userID,
		ActionType:  actionType,
		Payload:     string(data),
	}
	if err := db.Create(&entry).Error; err != nil {
		log.Printf("activity: create failed: %v", err)
	}
}

func ptr[T any](v T) *T {
	return &v
}
