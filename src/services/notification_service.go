package services

import (
	"log"
	"time"

	"collabpm/models"
	"collabpm/realtime"
	"collabpm/utils"

	"gorm.io/gorm"
)

type NotificationService struct {
	db *gorm.DB
}

func NewNotificationService(db *gorm.DB) *NotificationService {
	return &NotificationService{db: db}
}

type NotificationResponse struct {
	ID        uint      `json:"id"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	ProjectID *uint     `json:"project_id"`
	TaskID    *uint     `json:"task_id"`
	IsRead    bool      `json:"is_read"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *NotificationService) List(userID uint, unreadOnly bool) ([]NotificationResponse, error) {
	q := s.db.Where("user_id = ?", userID)
	if unreadOnly {
		q = q.Where("is_read = ?", false)
	}

	var notifs []models.Notification
	if err := q.Order("created_at DESC").Limit(100).Find(&notifs).Error; err != nil {
		return nil, utils.Internal()
	}

	out := make([]NotificationResponse, 0, len(notifs))
	for _, n := range notifs {
		out = append(out, toNotificationResponse(n))
	}
	return out, nil
}

func (s *NotificationService) MarkRead(userID, id uint) error {
	result := s.db.Model(&models.Notification{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("is_read", true)
	if result.Error != nil {
		return utils.Internal()
	}
	if result.RowsAffected == 0 {
		return utils.NotFound("notification not found")
	}
	return nil
}

func (s *NotificationService) MarkAllRead(userID uint) error {
	if err := s.db.Model(&models.Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Update("is_read", true).Error; err != nil {
		return utils.Internal()
	}
	return nil
}

// createNotification persists a notification and pushes it to the user's
// realtime room. Used by task, comment and project services.
func createNotification(db *gorm.DB, hub *realtime.Hub, userID uint, notifType, message string, projectID, taskID *uint) {
	notif := models.Notification{
		UserID:    userID,
		Type:      notifType,
		Message:   message,
		ProjectID: projectID,
		TaskID:    taskID,
		IsRead:    false,
	}
	if err := db.Create(&notif).Error; err != nil {
		log.Printf("notification: create failed: %v", err)
		return
	}
	hub.SendToUser(userID, realtime.Message{
		Type: "notification",
		Data: toNotificationResponse(notif),
	})
}

func toNotificationResponse(n models.Notification) NotificationResponse {
	return NotificationResponse{
		ID:        n.ID,
		Type:      n.Type,
		Message:   n.Message,
		ProjectID: n.ProjectID,
		TaskID:    n.TaskID,
		IsRead:    n.IsRead,
		CreatedAt: n.CreatedAt,
	}
}
