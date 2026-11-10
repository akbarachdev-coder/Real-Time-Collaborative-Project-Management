package services

import (
	"time"

	"collabpm/models"
	"collabpm/utils"

	"gorm.io/gorm"
)

type ActivityService struct {
	db *gorm.DB
}

func NewActivityService(db *gorm.DB) *ActivityService {
	return &ActivityService{db: db}
}

type ActivityResponse struct {
	ID          uint       `json:"id"`
	WorkspaceID uint       `json:"workspace_id"`
	ProjectID   *uint      `json:"project_id"`
	TaskID      *uint      `json:"task_id"`
	UserID      *uint      `json:"user_id"`
	ActionType  string     `json:"action_type"`
	Payload     string     `json:"payload"`
	CreatedAt   time.Time  `json:"created_at"`
	User        *UserBrief `json:"user,omitempty"`
}

func (s *ActivityService) ListByWorkspace(workspaceID, userID uint, limit int) ([]ActivityResponse, error) {
	if err := RequireWorkspaceMember(s.db, workspaceID, userID); err != nil {
		return nil, err
	}
	return s.query(ptr(workspaceID), nil, limit)
}

func (s *ActivityService) ListByTask(taskID, userID uint, limit int) ([]ActivityResponse, error) {
	var task models.Task
	if err := s.db.First(&task, taskID).Error; err != nil {
		return nil, utils.NotFound("task not found")
	}
	if _, err := workspaceIDForProject(s.db, task.ProjectID, userID); err != nil {
		return nil, err
	}
	return s.query(nil, ptr(taskID), limit)
}

func (s *ActivityService) query(workspaceID, taskID *uint, limit int) ([]ActivityResponse, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	q := s.db.Model(&models.ActivityLog{})
	if workspaceID != nil {
		q = q.Where("workspace_id = ?", *workspaceID)
	}
	if taskID != nil {
		q = q.Where("task_id = ?", *taskID)
	}

	var logs []models.ActivityLog
	if err := q.Order("created_at DESC, id DESC").
		Limit(limit).
		Find(&logs).Error; err != nil {
		return nil, utils.Internal()
	}

	userIDs := make([]uint, 0, len(logs))
	seen := make(map[uint]bool)
	for _, l := range logs {
		if l.UserID != nil && !seen[*l.UserID] {
			seen[*l.UserID] = true
			userIDs = append(userIDs, *l.UserID)
		}
	}
	userByID := make(map[uint]UserBrief, len(userIDs))
	if len(userIDs) > 0 {
		var users []models.User
		if err := s.db.Where("id IN ?", userIDs).Find(&users).Error; err != nil {
			return nil, utils.Internal()
		}
		for _, u := range users {
			userByID[u.ID] = UserBrief{ID: u.ID, Name: u.Name, Avatar: u.Avatar}
		}
	}

	out := make([]ActivityResponse, 0, len(logs))
	for _, l := range logs {
		item := ActivityResponse{
			ID:          l.ID,
			WorkspaceID: l.WorkspaceID,
			ProjectID:   l.ProjectID,
			TaskID:      l.TaskID,
			UserID:      l.UserID,
			ActionType:  l.ActionType,
			Payload:     l.Payload,
			CreatedAt:   l.CreatedAt,
		}
		if l.UserID != nil {
			if u, ok := userByID[*l.UserID]; ok {
				uu := u
				item.User = &uu
			}
		}
		out = append(out, item)
	}
	return out, nil
}
