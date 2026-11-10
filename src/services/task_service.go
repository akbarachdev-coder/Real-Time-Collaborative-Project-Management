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

// encodeJSONString stores a plain string as a JSON string value (schema: JSON column).
func encodeJSONString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return `""`
	}
	return string(b)
}

// decodeJSONString unwaps a JSON-encoded string; returns raw on failure.
func decodeJSONString(raw string) string {
	var s string
	if err := json.Unmarshal([]byte(raw), &s); err == nil {
		return s
	}
	return raw
}

type TaskService struct {
	db  *gorm.DB
	hub *realtime.Hub
}

func NewTaskService(db *gorm.DB, hub *realtime.Hub) *TaskService {
	return &TaskService{db: db, hub: hub}
}

type CreateTaskRequest struct {
	Title       string     `json:"title" binding:"required,max=500"`
	Description string     `json:"description" binding:"max=10000"`
	StatusID    *uint      `json:"status_id"`
	Priority    string     `json:"priority" binding:"omitempty,oneof=LOW MEDIUM HIGH URGENT"`
	AssigneeID  *uint      `json:"assignee_id"`
	DueDate     *time.Time `json:"due_date"`
}

type UpdateTaskRequest struct {
	Title       *string         `json:"title" binding:"omitempty,min=1,max=500"`
	Description *string         `json:"description" binding:"omitempty,max=10000"`
	StatusID    *uint           `json:"status_id"`
	Priority    *string         `json:"priority" binding:"omitempty,oneof=LOW MEDIUM HIGH URGENT"`
	Position    *float64        `json:"position"`
	AssigneeID  *utils.NullUint `json:"assignee_id"`
	DueDate     *utils.NullTime `json:"due_date"`
}

type UserBrief struct {
	ID     uint   `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

type TaskStatusBrief struct {
	ID    uint   `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type TaskResponse struct {
	ID          uint             `json:"id"`
	ProjectID   uint             `json:"project_id"`
	TaskNumber  int              `json:"task_number"`
	Title       string           `json:"title"`
	Description string           `json:"description"`
	StatusID    uint             `json:"status_id"`
	Status      *TaskStatusBrief `json:"status"`
	Priority    string           `json:"priority"`
	Position    float64          `json:"position"`
	Assignee    *UserBrief       `json:"assignee"`
	DueDate     *time.Time       `json:"due_date"`
	ReporterID  *uint            `json:"reporter_id"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

func (s *TaskService) ListByProject(projectID, userID uint) ([]TaskResponse, error) {
	if _, err := workspaceIDForProject(s.db, projectID, userID); err != nil {
		return nil, err
	}

	var tasks []models.Task
	err := s.db.Where("project_id = ?", projectID).
		Order("position ASC, id ASC").
		Find(&tasks).Error
	if err != nil {
		return nil, utils.Internal()
	}

	return s.buildResponses(projectID, tasks)
}

func (s *TaskService) Get(taskID, userID uint) (*TaskResponse, error) {
	task, workspaceID, err := s.load(taskID, userID)
	if err != nil {
		return nil, err
	}
	_ = workspaceID
	responses, err := s.buildResponses(task.ProjectID, []models.Task{*task})
	if err != nil {
		return nil, err
	}
	return &responses[0], nil
}

func (s *TaskService) Create(projectID, userID uint, req CreateTaskRequest) (*TaskResponse, error) {
	workspaceID, err := workspaceIDForProject(s.db, projectID, userID)
	if err != nil {
		return nil, err
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, utils.BadRequest("title cannot be empty")
	}

	priority := req.Priority
	if priority == "" {
		priority = models.PriorityMedium
	}
	if !models.IsValidPriority(priority) {
		return nil, utils.BadRequest("invalid priority")
	}

	statusID, err := s.resolveStatusID(projectID, req.StatusID)
	if err != nil {
		return nil, err
	}

	if req.AssigneeID != nil {
		if err := s.validateAssignee(workspaceID, *req.AssigneeID); err != nil {
			return nil, err
		}
	}

	var task models.Task
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var maxNum int
		if err := tx.Model(&models.Task{}).
			Where("project_id = ?", projectID).
			Select("COALESCE(MAX(task_number), 0)").
			Scan(&maxNum).Error; err != nil {
			return err
		}

		var maxPos float64
		if err := tx.Model(&models.Task{}).
			Where("project_id = ? AND status_id = ?", projectID, statusID).
			Select("COALESCE(MAX(position), 0)").
			Scan(&maxPos).Error; err != nil {
			return err
		}

		task = models.Task{
			ProjectID:   projectID,
			StatusID:    statusID,
			TaskNumber:  maxNum + 1,
			Title:       title,
			Description: encodeJSONString(req.Description),
			Priority:    priority,
			Position:    maxPos + 1,
			DueDate:     req.DueDate,
			ReporterID:  ptr(userID),
		}
		if err := tx.Create(&task).Error; err != nil {
			return err
		}
		if req.AssigneeID != nil {
			return tx.Create(&models.TaskAssignee{
				TaskID: task.ID,
				UserID: *req.AssigneeID,
			}).Error
		}
		return nil
	})
	if err != nil {
		return nil, utils.Internal()
	}

	responses, err := s.buildResponses(projectID, []models.Task{task})
	if err != nil {
		return nil, err
	}
	resp := responses[0]

	LogActivity(s.db, workspaceID, ptr(projectID), ptr(task.ID), ptr(userID), "task.created", map[string]any{
		"task_id":     task.ID,
		"task_number": task.TaskNumber,
		"title":       task.Title,
	})
	s.hub.BroadcastToProject(projectID, realtime.Message{
		Type:      "task.created",
		ProjectID: projectID,
		Data:      resp,
	})
	if task.ReporterID != nil && resp.Assignee != nil && resp.Assignee.ID != userID {
		s.notifyAssigned(projectID, task, resp.Assignee.ID)
	}
	return &resp, nil
}

func (s *TaskService) Update(taskID, userID uint, req UpdateTaskRequest) (*TaskResponse, error) {
	task, workspaceID, err := s.load(taskID, userID)
	if err != nil {
		return nil, err
	}

	oldStatusID := task.StatusID
	oldAssignee := s.primaryAssignee(task.ID)

	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			return nil, utils.BadRequest("title cannot be empty")
		}
		task.Title = title
	}
	if req.Description != nil {
		task.Description = encodeJSONString(*req.Description)
	}
	if req.StatusID != nil {
		statusID, err := s.resolveStatusID(task.ProjectID, req.StatusID)
		if err != nil {
			return nil, err
		}
		task.StatusID = statusID
	}
	if req.Priority != nil {
		if !models.IsValidPriority(*req.Priority) {
			return nil, utils.BadRequest("invalid priority")
		}
		task.Priority = *req.Priority
	}
	if req.DueDate != nil {
		if req.DueDate.Valid {
			t := req.DueDate.Value
			task.DueDate = &t
		} else {
			task.DueDate = nil
		}
	}

	if req.AssigneeID != nil {
		if req.AssigneeID.Valid {
			if err := s.validateAssignee(workspaceID, req.AssigneeID.Value); err != nil {
				return nil, err
			}
			v := req.AssigneeID.Value
			if err := s.db.Transaction(func(tx *gorm.DB) error {
				if err := tx.Where("task_id = ?", task.ID).Delete(&models.TaskAssignee{}).Error; err != nil {
					return err
				}
				return tx.Create(&models.TaskAssignee{TaskID: task.ID, UserID: v}).Error
			}); err != nil {
				return nil, utils.Internal()
			}
		} else {
			if err := s.db.Where("task_id = ?", task.ID).Delete(&models.TaskAssignee{}).Error; err != nil {
				return nil, utils.Internal()
			}
		}
	}

	if req.Position != nil {
		task.Position = *req.Position
	} else if req.StatusID != nil && task.StatusID != oldStatusID {
		var maxPos float64
		if err := s.db.Model(&models.Task{}).
			Where("project_id = ? AND status_id = ?", task.ProjectID, task.StatusID).
			Select("COALESCE(MAX(position), 0)").
			Scan(&maxPos).Error; err != nil {
			return nil, utils.Internal()
		}
		task.Position = maxPos + 1
	}

	if err := s.db.Save(task).Error; err != nil {
		return nil, utils.Internal()
	}

	responses, err := s.buildResponses(task.ProjectID, []models.Task{*task})
	if err != nil {
		return nil, err
	}
	resp := responses[0]

	LogActivity(s.db, workspaceID, ptr(task.ProjectID), ptr(task.ID), ptr(userID), "task.updated", map[string]any{
		"task_id": task.ID,
		"fields":  taskUpdateFields(req, oldStatusID, task),
	})
	s.hub.BroadcastToProject(task.ProjectID, realtime.Message{
		Type:      "task.updated",
		ProjectID: task.ProjectID,
		Data:      resp,
	})

	newAssignee := uint(0)
	if resp.Assignee != nil {
		newAssignee = resp.Assignee.ID
	}
	if newAssignee != 0 && newAssignee != oldAssignee && newAssignee != userID {
		s.notifyAssigned(task.ProjectID, *task, newAssignee)
	}
	return &resp, nil
}

func (s *TaskService) Delete(taskID, userID uint) error {
	task, workspaceID, err := s.load(taskID, userID)
	if err != nil {
		return err
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("task_id = ?", task.ID).Delete(&models.Comment{}).Error; err != nil {
			return err
		}
		if err := tx.Where("task_id = ?", task.ID).Delete(&models.TaskAssignee{}).Error; err != nil {
			return err
		}
		if err := tx.Where("task_id = ?", task.ID).Delete(&models.TaskDocState{}).Error; err != nil {
			return err
		}
		if err := tx.Where("task_id = ?", task.ID).Delete(&models.Notification{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.Task{}, task.ID).Error
	})
	if err != nil {
		return utils.Internal()
	}

	LogActivity(s.db, workspaceID, ptr(task.ProjectID), ptr(task.ID), ptr(userID), "task.deleted", map[string]any{
		"task_id":     task.ID,
		"task_number": task.TaskNumber,
	})
	s.hub.BroadcastToProject(task.ProjectID, realtime.Message{
		Type:      "task.deleted",
		ProjectID: task.ProjectID,
		Data:      map[string]any{"id": task.ID},
	})
	return nil
}

func (s *TaskService) load(taskID, userID uint) (*models.Task, uint, error) {
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

func (s *TaskService) resolveStatusID(projectID uint, statusID *uint) (uint, error) {
	if statusID != nil {
		var count int64
		if err := s.db.Model(&models.ProjectStatus{}).
			Where("id = ? AND project_id = ?", *statusID, projectID).
			Count(&count).Error; err != nil {
			return 0, utils.Internal()
		}
		if count == 0 {
			return 0, utils.BadRequest("invalid status_id for this project")
		}
		return *statusID, nil
	}

	var first models.ProjectStatus
	err := s.db.Where("project_id = ?", projectID).
		Order("position ASC, id ASC").
		First(&first).Error
	if err == gorm.ErrRecordNotFound {
		return 0, utils.BadRequest("project has no statuses configured")
	}
	if err != nil {
		return 0, utils.Internal()
	}
	return first.ID, nil
}

func (s *TaskService) validateAssignee(workspaceID, assigneeID uint) error {
	var count int64
	if err := s.db.Model(&models.WorkspaceMember{}).
		Where("workspace_id = ? AND user_id = ?", workspaceID, assigneeID).
		Count(&count).Error; err != nil {
		return utils.Internal()
	}
	if count == 0 {
		return utils.BadRequest("assignee must be a member of the workspace")
	}
	return nil
}

func (s *TaskService) primaryAssignee(taskID uint) uint {
	var assignee models.TaskAssignee
	if err := s.db.Where("task_id = ?", taskID).First(&assignee).Error; err != nil {
		return 0
	}
	return assignee.UserID
}

// buildResponses loads statuses + assignees for a batch of tasks.
func (s *TaskService) buildResponses(projectID uint, tasks []models.Task) ([]TaskResponse, error) {
	if len(tasks) == 0 {
		return []TaskResponse{}, nil
	}

	var statuses []models.ProjectStatus
	if err := s.db.Where("project_id = ?", projectID).Find(&statuses).Error; err != nil {
		return nil, utils.Internal()
	}
	statusByID := make(map[uint]models.ProjectStatus, len(statuses))
	for _, st := range statuses {
		statusByID[st.ID] = st
	}

	taskIDs := make([]uint, 0, len(tasks))
	for _, t := range tasks {
		taskIDs = append(taskIDs, t.ID)
	}

	var assigneeRows []struct {
		TaskID uint
		UserID uint
	}
	if err := s.db.Model(&models.TaskAssignee{}).
		Where("task_id IN ?", taskIDs).
		Find(&assigneeRows).Error; err != nil {
		return nil, utils.Internal()
	}

	userIDs := make([]uint, 0, len(assigneeRows))
	seen := make(map[uint]bool, len(assigneeRows))
	for _, r := range assigneeRows {
		if !seen[r.UserID] {
			seen[r.UserID] = true
			userIDs = append(userIDs, r.UserID)
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

	assigneeByTask := make(map[uint]UserBrief, len(assigneeRows))
	for _, r := range assigneeRows {
		if u, ok := userByID[r.UserID]; ok {
			if _, exists := assigneeByTask[r.TaskID]; !exists {
				assigneeByTask[r.TaskID] = u
			}
		}
	}

	out := make([]TaskResponse, 0, len(tasks))
	for _, t := range tasks {
		resp := TaskResponse{
			ID:          t.ID,
			ProjectID:   t.ProjectID,
			TaskNumber:  t.TaskNumber,
			Title:       t.Title,
			Description: decodeJSONString(t.Description),
			StatusID:    t.StatusID,
			Priority:    t.Priority,
			Position:    t.Position,
			DueDate:     t.DueDate,
			ReporterID:  t.ReporterID,
			CreatedAt:   t.CreatedAt,
			UpdatedAt:   t.UpdatedAt,
		}
		if st, ok := statusByID[t.StatusID]; ok {
			resp.Status = &TaskStatusBrief{ID: st.ID, Name: st.Name, Color: st.Color}
		}
		if a, ok := assigneeByTask[t.ID]; ok {
			aa := a
			resp.Assignee = &aa
		}
		out = append(out, resp)
	}
	return out, nil
}

func (s *TaskService) notifyAssigned(projectID uint, task models.Task, assigneeID uint) {
	var project models.Project
	title := fmt.Sprintf("You were assigned to task #%d", task.TaskNumber)
	if err := s.db.First(&project, projectID).Error; err == nil {
		title = fmt.Sprintf("You were assigned to %s-%d", project.Identifier, task.TaskNumber)
	}
	createNotification(s.db, s.hub, assigneeID, models.NotificationTaskAssigned,
		title, ptr(projectID), ptr(task.ID))
}

func taskUpdateFields(req UpdateTaskRequest, oldStatusID uint, task *models.Task) map[string]any {
	fields := map[string]any{}
	if req.Title != nil {
		fields["title"] = task.Title
	}
	if req.Description != nil {
		fields["description"] = true
	}
	if req.StatusID != nil && task.StatusID != oldStatusID {
		fields["status_id"] = task.StatusID
	}
	if req.Priority != nil {
		fields["priority"] = task.Priority
	}
	if req.Position != nil {
		fields["position"] = task.Position
	}
	if req.AssigneeID != nil {
		fields["assignee_id"] = req.AssigneeID
	}
	if req.DueDate != nil {
		fields["due_date"] = task.DueDate
	}
	return fields
}
