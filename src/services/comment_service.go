package services

import (
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
	Content string `json:"content" binding:"required,max=5000"`
}

type CommentResponse struct {
	ID        uint      `json:"id"`
	TaskID    uint      `json:"task_id"`
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
		Content   string
		CreatedAt time.Time
		UpdatedAt time.Time
		UserID    uint
		Name      string
		Avatar    string
	}
	err = s.db.Table("comments").
		Select("comments.id, comments.task_id, comments.content, comments.created_at, comments.updated_at, comments.user_id, users.name, users.avatar").
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
			Content:   r.Content,
			User:      UserBrief{ID: r.UserID, Name: r.Name, Avatar: r.Avatar},
			CreatedAt: r.CreatedAt,
			UpdatedAt: r.UpdatedAt,
		})
	}
	return out, nil
}

func (s *CommentService) Create(taskID, userID uint, req CreateCommentRequest) (*CommentResponse, error) {
	task, err := s.loadTask(taskID, userID)
	if err != nil {
		return nil, err
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, utils.BadRequest("comment content cannot be empty")
	}

	var actor models.User
	if err := s.db.First(&actor, userID).Error; err != nil {
		return nil, utils.Internal()
	}

	comment := models.Comment{
		TaskID:  task.ID,
		UserID:  userID,
		Content: content,
	}
	if err := s.db.Create(&comment).Error; err != nil {
		return nil, utils.Internal()
	}

	resp := &CommentResponse{
		ID:        comment.ID,
		TaskID:    comment.TaskID,
		Content:   comment.Content,
		User:      UserBrief{ID: actor.ID, Name: actor.Name, Avatar: actor.Avatar},
		CreatedAt: comment.CreatedAt,
		UpdatedAt: comment.UpdatedAt,
	}

	s.hub.BroadcastToProject(task.ProjectID, realtime.Message{
		Type:      "comment.created",
		ProjectID: task.ProjectID,
		Data:      resp,
	})

	message := fmt.Sprintf("%s commented on task %q", actor.Name, task.Title)
	projectID := task.ProjectID
	taskIDVal := task.ID
	notified := map[uint]bool{userID: true}
	if task.AssigneeID != nil && !notified[*task.AssigneeID] {
		notified[*task.AssigneeID] = true
		createNotification(s.db, s.hub, *task.AssigneeID, models.NotificationCommentAdded,
			message, &projectID, &taskIDVal)
	}
	if !notified[task.CreatedBy] {
		createNotification(s.db, s.hub, task.CreatedBy, models.NotificationCommentAdded,
			message, &projectID, &taskIDVal)
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

	var task models.Task
	if err := s.db.First(&task, comment.TaskID).Error; err != nil {
		return utils.Internal()
	}
	if err := RequireProjectMember(s.db, task.ProjectID, userID); err != nil {
		return err
	}

	if err := s.db.Delete(&comment).Error; err != nil {
		return utils.Internal()
	}

	s.hub.BroadcastToProject(task.ProjectID, realtime.Message{
		Type:      "comment.deleted",
		ProjectID: task.ProjectID,
		Data:      map[string]any{"id": comment.ID, "task_id": comment.TaskID},
	})
	return nil
}

func (s *CommentService) loadTask(taskID, userID uint) (*models.Task, error) {
	var task models.Task
	if err := s.db.First(&task, taskID).Error; err != nil {
		return nil, utils.NotFound("task not found")
	}
	if err := RequireProjectMember(s.db, task.ProjectID, userID); err != nil {
		return nil, err
	}
	return &task, nil
}
