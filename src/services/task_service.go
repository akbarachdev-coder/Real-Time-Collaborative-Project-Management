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

var validStatuses = map[string]bool{
	models.TaskStatusTodo:       true,
	models.TaskStatusInProgress: true,
	models.TaskStatusReview:     true,
	models.TaskStatusDone:       true,
}

type TaskService struct {
	db  *gorm.DB
	hub *realtime.Hub
}

func NewTaskService(db *gorm.DB, hub *realtime.Hub) *TaskService {
	return &TaskService{db: db, hub: hub}
}

type CreateTaskRequest struct {
	Title       string     `json:"title" binding:"required,max=200"`
	Description string     `json:"description" binding:"max=10000"`
	Status      string     `json:"status" binding:"omitempty,oneof=todo in_progress review done"`
	AssigneeID  *uint      `json:"assignee_id"`
	DueDate     *time.Time `json:"due_date"`
}

type UpdateTaskRequest struct {
	Title       *string          `json:"title" binding:"omitempty,max=200"`
	Description *string          `json:"description" binding:"omitempty,max=10000"`
	Status      *string          `json:"status" binding:"omitempty,oneof=todo in_progress review done"`
	Position    *float64         `json:"position"`
	AssigneeID  *utils.NullUint  `json:"assignee_id"`
	DueDate     *utils.NullTime  `json:"due_date"`
}

type UserBrief struct {
	ID     uint   `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

type TaskResponse struct {
	ID          uint       `json:"id"`
	ProjectID   uint       `json:"project_id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	Position    float64    `json:"position"`
	Assignee    *UserBrief `json:"assignee"`
	DueDate     *time.Time `json:"due_date"`
	CreatedBy   uint       `json:"created_by"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (s *TaskService) ListByProject(projectID, userID uint) ([]TaskResponse, error) {
	if err := RequireProjectMember(s.db, projectID, userID); err != nil {
		return nil, err
	}

	var tasks []models.Task
	err := s.db.Where("project_id = ?", projectID).
		Order("position ASC, id ASC").
		Find(&tasks).Error
	if err != nil {
		return nil, utils.Internal()
	}

	assignees, err := s.loadAssignees(tasks)
	if err != nil {
		return nil, err
	}

	out := make([]TaskResponse, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, buildTaskResponse(t, assignees))
	}
	return out, nil
}

func (s *TaskService) Get(taskID, userID uint) (*TaskResponse, error) {
	task, err := s.load(taskID, userID)
	if err != nil {
		return nil, err
	}
	assignees, err := s.loadAssignees([]models.Task{*task})
	if err != nil {
		return nil, err
	}
	resp := buildTaskResponse(*task, assignees)
	return &resp, nil
}

func (s *TaskService) Create(projectID, userID uint, req CreateTaskRequest) (*TaskResponse, error) {
	if err := RequireProjectMember(s.db, projectID, userID); err != nil {
		return nil, err
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, utils.BadRequest("title cannot be empty")
	}

	status := req.Status
	if status == "" {
		status = models.TaskStatusTodo
	}
	if !validStatuses[status] {
		return nil, utils.BadRequest("invalid status")
	}

	if req.AssigneeID != nil {
		if err := s.validateAssignee(projectID, *req.AssigneeID); err != nil {
			return nil, err
		}
	}

	pos, err := s.maxPosition(projectID, status)
	if err != nil {
		return nil, utils.Internal()
	}

	task := models.Task{
		ProjectID:   projectID,
		Title:       title,
		Description: req.Description,
		Status:      status,
		Position:    pos + 1,
		AssigneeID:  req.AssigneeID,
		DueDate:     req.DueDate,
		CreatedBy:   userID,
	}
	if err := s.db.Create(&task).Error; err != nil {
		return nil, utils.Internal()
	}

	assignees, err := s.loadAssignees([]models.Task{task})
	if err != nil {
		return nil, err
	}
	resp := buildTaskResponse(task, assignees)

	s.hub.BroadcastToProject(projectID, realtime.Message{
		Type:      "task.created",
		ProjectID: projectID,
		Data:      resp,
	})
	if task.AssigneeID != nil && *task.AssigneeID != userID {
		s.notifyAssigned(task)
	}
	return &resp, nil
}

func (s *TaskService) Update(taskID, userID uint, req UpdateTaskRequest) (*TaskResponse, error) {
	task, err := s.load(taskID, userID)
	if err != nil {
		return nil, err
	}

	oldStatus := task.Status
	oldAssignee := task.AssigneeID

	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			return nil, utils.BadRequest("title cannot be empty")
		}
		task.Title = title
	}
	if req.Description != nil {
		task.Description = *req.Description
	}
	if req.Status != nil {
		if !validStatuses[*req.Status] {
			return nil, utils.BadRequest("invalid status")
		}
		task.Status = *req.Status
	}
	if req.AssigneeID != nil {
		if req.AssigneeID.Valid {
			if err := s.validateAssignee(task.ProjectID, req.AssigneeID.Value); err != nil {
				return nil, err
			}
			v := req.AssigneeID.Value
			task.AssigneeID = &v
		} else {
			task.AssigneeID = nil
		}
	}
	if req.DueDate != nil {
		if req.DueDate.Valid {
			t := req.DueDate.Value
			task.DueDate = &t
		} else {
			task.DueDate = nil
		}
	}

	if req.Position != nil {
		task.Position = *req.Position
	} else if req.Status != nil && task.Status != oldStatus {
		pos, err := s.maxPosition(task.ProjectID, task.Status)
		if err != nil {
			return nil, utils.Internal()
		}
		task.Position = pos + 1
	}

	if err := s.db.Save(task).Error; err != nil {
		return nil, utils.Internal()
	}

	assignees, err := s.loadAssignees([]models.Task{*task})
	if err != nil {
		return nil, err
	}
	resp := buildTaskResponse(*task, assignees)

	s.hub.BroadcastToProject(task.ProjectID, realtime.Message{
		Type:      "task.updated",
		ProjectID: task.ProjectID,
		Data:      resp,
	})

	if !sameUint(task.AssigneeID, oldAssignee) && task.AssigneeID != nil && *task.AssigneeID != userID {
		s.notifyAssigned(*task)
	}
	return &resp, nil
}

func (s *TaskService) Delete(taskID, userID uint) error {
	task, err := s.load(taskID, userID)
	if err != nil {
		return err
	}
	if err := s.db.Delete(task).Error; err != nil {
		return utils.Internal()
	}
	s.hub.BroadcastToProject(task.ProjectID, realtime.Message{
		Type:      "task.deleted",
		ProjectID: task.ProjectID,
		Data:      map[string]any{"id": task.ID},
	})
	return nil
}

func (s *TaskService) load(taskID, userID uint) (*models.Task, error) {
	var task models.Task
	if err := s.db.First(&task, taskID).Error; err != nil {
		return nil, utils.NotFound("task not found")
	}
	if err := RequireProjectMember(s.db, task.ProjectID, userID); err != nil {
		return nil, err
	}
	return &task, nil
}

func (s *TaskService) validateAssignee(projectID, assigneeID uint) error {
	ok, err := IsProjectMember(s.db, projectID, assigneeID)
	if err != nil {
		return utils.Internal()
	}
	if !ok {
		return utils.BadRequest("assignee must be a member of the project")
	}
	return nil
}

func (s *TaskService) maxPosition(projectID uint, status string) (float64, error) {
	var pos float64
	err := s.db.Model(&models.Task{}).
		Where("project_id = ? AND status = ?", projectID, status).
		Select("COALESCE(MAX(position), 0)").
		Scan(&pos).Error
	return pos, err
}

func (s *TaskService) loadAssignees(tasks []models.Task) (map[uint]*UserBrief, error) {
	out := make(map[uint]*UserBrief)
	ids := make([]uint, 0, len(tasks))
	seen := make(map[uint]bool)
	for _, t := range tasks {
		if t.AssigneeID != nil && !seen[*t.AssigneeID] {
			seen[*t.AssigneeID] = true
			ids = append(ids, *t.AssigneeID)
		}
	}
	if len(ids) == 0 {
		return out, nil
	}

	var users []models.User
	if err := s.db.Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, utils.Internal()
	}
	for _, u := range users {
		out[u.ID] = &UserBrief{ID: u.ID, Name: u.Name, Avatar: u.Avatar}
	}
	return out, nil
}

func (s *TaskService) notifyAssigned(task models.Task) {
	projectID := task.ProjectID
	taskID := task.ID
	createNotification(s.db, s.hub, *task.AssigneeID, models.NotificationTaskAssigned,
		fmt.Sprintf("You were assigned to task %q", task.Title),
		&projectID, &taskID)
}

func buildTaskResponse(t models.Task, assignees map[uint]*UserBrief) TaskResponse {
	resp := TaskResponse{
		ID:          t.ID,
		ProjectID:   t.ProjectID,
		Title:       t.Title,
		Description: t.Description,
		Status:      t.Status,
		Position:    t.Position,
		DueDate:     t.DueDate,
		CreatedBy:   t.CreatedBy,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
	if t.AssigneeID != nil {
		if a, ok := assignees[*t.AssigneeID]; ok {
			resp.Assignee = a
		}
	}
	return resp
}

func sameUint(a, b *uint) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
