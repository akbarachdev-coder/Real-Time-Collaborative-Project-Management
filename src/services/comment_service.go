package services

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"collabpm/models"
	"collabpm/realtime"
	"collabpm/utils"

	"gorm.io/gorm"
)

type CommentService struct {
	db  *gorm.DB
	hub *realtime.Hub
}

func NewCommentService(db *gorm.DB, hub *realtime.Hub) *CommentService {
	return &CommentService{db: db, hub: hub}
}

type CreateCommentRequest struct {
	Content  string `json:"content" binding:"required,max=5000"`
	ParentID *uint  `json:"parent_id"`
}

type CommentResponse struct {
	ID        uint      `json:"id"`
	TaskID    uint      `json:"task_id"`
	ParentID  *uint     `json:"parent_id"`
	Content   string    `json:"content"`
	User      UserBrief `json:"user"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s *CommentService) List(taskID, userID uint) ([]CommentResponse, error) {
	task, err := s.loadTask(taskID, userID)
	if err != nil {
		return nil, err
	}

	var rows []struct {
		ID        uint
		TaskID    uint
		ParentID  *uint
		Content   string
		CreatedAt time.Time
		UpdatedAt time.Time
		UserID    uint
		Name      string
		Avatar    string
	}
	err = s.db.Table("comments").
		Select("comments.id, comments.task_id, comments.parent_id, comments.content, comments.created_at, comments.updated_at, comments.user_id, users.name, users.avatar").
		Joins("JOIN users ON users.id = comments.user_id").
		Where("comments.task_id = ?", task.ID).
		Order("comments.created_at ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, utils.Internal()
	}

	out := make([]CommentResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, CommentResponse{
			ID:        r.ID,
			TaskID:    r.TaskID,
			ParentID:  r.ParentID,
			Content:   decodeCommentContent(r.Content),
			User:      UserBrief{ID: r.UserID, Name: r.Name, Avatar: r.Avatar},
			CreatedAt: r.CreatedAt,
			UpdatedAt: r.UpdatedAt,
		})
	}
	return out, nil
}

func (s *CommentService) Create(taskID, userID uint, req CreateCommentRequest) (*CommentResponse, error) {
	task, workspaceID, err := s.loadTaskWithWorkspace(taskID, userID)
	if err != nil {
		return nil, err
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, utils.BadRequest("comment content cannot be empty")
	}

	// Store content as a JSON string value (schema: content JSON NOT NULL).
	encoded, err := json.Marshal(content)
	if err != nil {
		return nil, utils.Internal()
	}

	if req.ParentID != nil {
		var parentCount int64
		if err := s.db.Model(&models.Comment{}).
			Where("id = ? AND task_id = ?", *req.ParentID, task.ID).
			Count(&parentCount).Error; err != nil {
			return nil, utils.Internal()
		}
		if parentCount == 0 {
			return nil, utils.BadRequest("parent comment not found on this task")
		}
	}

	var actor models.User
	if err := s.db.First(&actor, userID).Error; err != nil {
		return nil, utils.Internal()
	}

	comment := models.Comment{
		TaskID:   task.ID,
		UserID:   userID,
		ParentID: req.ParentID,
		Content:  string(encoded),
	}
	if err := s.db.Create(&comment).Error; err != nil {
		return nil, utils.Internal()
	}

	resp := &CommentResponse{
		ID:        comment.ID,
		TaskID:    comment.TaskID,
		ParentID:  comment.ParentID,
		Content:   content,
		User:      UserBrief{ID: actor.ID, Name: actor.Name, Avatar: actor.Avatar},
		CreatedAt: comment.CreatedAt,
		UpdatedAt: comment.UpdatedAt,
	}

	LogActivity(s.db, workspaceID, ptr(task.ProjectID), ptr(task.ID), ptr(userID), "comment.created", map[string]any{
		"comment_id": comment.ID,
	})

	s.hub.BroadcastToProject(task.ProjectID, realtime.Message{
		Type:      "comment.created",
		ProjectID: task.ProjectID,
		Data:      resp,
	})

	message := fmt.Sprintf("%s commented on task #%d", actor.Name, task.TaskNumber)
	notified := map[uint]bool{userID: true}
	for _, uid := range s.assigneeIDs(task.ID) {
		if !notified[uid] {
			notified[uid] = true
			createNotification(s.db, s.hub, uid, models.NotificationCommentAdded,
				message, ptr(task.ProjectID), ptr(task.ID))
		}
	}
	if task.ReporterID != nil && !notified[*task.ReporterID] {
		createNotification(s.db, s.hub, *task.ReporterID, models.NotificationCommentAdded,
			message, ptr(task.ProjectID), ptr(task.ID))
	}
	return resp, nil
}

func (s *CommentService) Delete(commentID, userID uint) error {
	var comment models.Comment
	if err := s.db.First(&comment, commentID).Error; err != nil {
		return utils.NotFound("comment not found")
	}
	if comment.UserID != userID {
		return utils.Forbidden("you can only delete your own comments")
	}

	task, workspaceID, err := s.loadTaskWithWorkspace(comment.TaskID, userID)
	if err != nil {
		return err
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("parent_id = ?", comment.ID).Delete(&models.Comment{}).Error; err != nil {
			return err
		}
		return tx.Delete(&comment).Error
	})
	if err != nil {
		return utils.Internal()
	}

	LogActivity(s.db, workspaceID, ptr(task.ProjectID), ptr(task.ID), ptr(userID), "comment.deleted", map[string]any{
		"comment_id": comment.ID,
	})

	s.hub.BroadcastToProject(task.ProjectID, realtime.Message{
		Type:      "comment.deleted",
		ProjectID: task.ProjectID,
		Data:      map[string]any{"id": comment.ID, "task_id": comment.TaskID},
	})
	return nil
}

func (s *CommentService) loadTask(taskID, userID uint) (*models.Task, error) {
	task, _, err := s.loadTaskWithWorkspace(taskID, userID)
	return task, err
}

func (s *CommentService) loadTaskWithWorkspace(taskID, userID uint) (*models.Task, uint, error) {
	var task models.Task
	if err := s.db.First(&task, taskID).Error; err != nil {
		return nil, 0, utils.NotFound("task not found")
	}
	workspaceID, err := workspaceIDForProject(s.db, task.ProjectID, userID)
	if err != nil {
		return nil, 0, err
	}
	return &task, workspaceID, nil
}

func (s *CommentService) assigneeIDs(taskID uint) []uint {
	var rows []uint
	if err := s.db.Model(&models.TaskAssignee{}).
		Where("task_id = ?", taskID).
		Pluck("user_id", &rows).Error; err != nil {
		return nil
	}
	return rows
}

// decodeCommentContent unwraps a JSON-encoded string stored in content.
func decodeCommentContent(raw string) string {
	var s string
	if err := json.Unmarshal([]byte(raw), &s); err == nil {
		return s
	}
	return raw
}
