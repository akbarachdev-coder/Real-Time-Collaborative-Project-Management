package services

import (
	"strings"
	"time"

	"collabpm/models"
	"collabpm/realtime"
	"collabpm/utils"

	"gorm.io/gorm"
)

type ProjectService struct {
	db  *gorm.DB
	hub *realtime.Hub
}

func NewProjectService(db *gorm.DB, hub *realtime.Hub) *ProjectService {
	return &ProjectService{db: db, hub: hub}
}

type CreateProjectRequest struct {
	WorkspaceID uint    `json:"workspace_id" binding:"required"`
	Name        string  `json:"name" binding:"required,max=255"`
	Identifier  string  `json:"identifier" binding:"required,max=10"`
	Description *string `json:"description" binding:"omitempty,max=5000"`
	IsPrivate   bool    `json:"is_private"`
}

type UpdateProjectRequest struct {
	Name        *string `json:"name" binding:"omitempty,min=1,max=255"`
	Description *string `json:"description" binding:"omitempty,max=5000"`
	IsPrivate   *bool   `json:"is_private"`
}

type ProjectSummary struct {
	ID          uint      `json:"id"`
	WorkspaceID uint      `json:"workspace_id"`
	Name        string    `json:"name"`
	Identifier  string    `json:"identifier"`
	Description *string   `json:"description"`
	IsPrivate   bool      `json:"is_private"`
	CreatedBy   *uint     `json:"created_by"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
}

type ProjectDetail struct {
	ID          uint             `json:"id"`
	WorkspaceID uint             `json:"workspace_id"`
	Name        string           `json:"name"`
	Identifier  string           `json:"identifier"`
	Description *string          `json:"description"`
	IsPrivate   bool             `json:"is_private"`
	CreatedBy   *uint            `json:"created_by"`
	Role        string           `json:"role"`
	Statuses    []StatusResponse `json:"statuses"`
	CreatedAt   time.Time        `json:"created_at"`
}

type StatusResponse struct {
	ID        uint      `json:"id"`
	ProjectID uint      `json:"project_id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	Position  float64   `json:"position"`
	CreatedAt time.Time `json:"created_at"`
}

var defaultStatuses = []struct {
	Name     string
	Color    string
	Position float64
}{
	{"Backlog", "#94a3b8", 10},
	{"Todo", "#60a5fa", 20},
	{"In Progress", "#fbbf24", 30},
	{"Done", "#34d399", 40},
}

func (s *ProjectService) List(userID uint) ([]ProjectSummary, error) {
	var rows []struct {
		ID          uint
		WorkspaceID uint
		Name        string
		Identifier  string
		Description *string
		IsPrivate   bool
		CreatedBy   *uint
		Role        string
		CreatedAt   time.Time
	}
	err := s.db.Table("projects").
		Select("projects.id, projects.workspace_id, projects.name, projects.identifier, projects.description, projects.is_private, projects.created_by, workspace_members.role, projects.created_at").
		Joins("JOIN workspace_members ON workspace_members.workspace_id = projects.workspace_id").
		Where("workspace_members.user_id = ?", userID).
		Order("projects.created_at DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, utils.Internal()
	}
	out := make([]ProjectSummary, 0, len(rows))
	for _, r := range rows {
		out = append(out, ProjectSummary{
			ID:          r.ID,
			WorkspaceID: r.WorkspaceID,
			Name:        r.Name,
			Identifier:  r.Identifier,
			Description: r.Description,
			IsPrivate:   r.IsPrivate,
			CreatedBy:   r.CreatedBy,
			Role:        r.Role,
			CreatedAt:   r.CreatedAt,
		})
	}
	return out, nil
}

func (s *ProjectService) Create(userID uint, req CreateProjectRequest) (*ProjectDetail, error) {
	if err := RequireWorkspaceRole(s.db, req.WorkspaceID, userID, models.RoleMember); err != nil {
		return nil, err
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, utils.BadRequest("project name cannot be empty")
	}

	identifier := strings.ToUpper(strings.TrimSpace(req.Identifier))
	if identifier == "" {
		return nil, utils.BadRequest("identifier cannot be empty")
	}
	for _, r := range identifier {
		if !(r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return nil, utils.BadRequest("identifier must be alphanumeric")
		}
	}

	var project models.Project
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&models.Project{}).
			Where("workspace_id = ? AND identifier = ?", req.WorkspaceID, identifier).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return utils.Conflict("identifier already exists in this workspace")
		}

		project = models.Project{
			WorkspaceID: req.WorkspaceID,
			Name:        name,
			Identifier:  identifier,
			Description: req.Description,
			IsPrivate:   req.IsPrivate,
			CreatedBy:   ptr(userID),
		}
		if err := tx.Create(&project).Error; err != nil {
			return err
		}
		for _, st := range defaultStatuses {
			if err := tx.Create(&models.ProjectStatus{
				ProjectID: project.ID,
				Name:      st.Name,
				Color:     st.Color,
				Position:  st.Position,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		if ae, ok := err.(*utils.AppError); ok {
			return nil, ae
		}
		return nil, utils.Internal()
	}

	LogActivity(s.db, req.WorkspaceID, ptr(project.ID), nil, ptr(userID), "project.created", map[string]any{
		"project_id": project.ID,
		"name":       project.Name,
		"identifier": project.Identifier,
	})
	return s.Get(project.ID, userID)
}

func (s *ProjectService) Get(projectID, userID uint) (*ProjectDetail, error) {
	workspaceID, err := workspaceIDForProject(s.db, projectID, userID)
	if err != nil {
		return nil, err
	}

	var project models.Project
	if err := s.db.First(&project, projectID).Error; err != nil {
		return nil, utils.NotFound("project not found")
	}

	role, err := WorkspaceMembership(s.db, workspaceID, userID)
	if err != nil {
		return nil, err
	}

	statuses, err := s.listStatuses(projectID)
	if err != nil {
		return nil, err
	}

	return &ProjectDetail{
		ID:          project.ID,
		WorkspaceID: project.WorkspaceID,
		Name:        project.Name,
		Identifier:  project.Identifier,
		Description: project.Description,
		IsPrivate:   project.IsPrivate,
		CreatedBy:   project.CreatedBy,
		Role:        role,
		Statuses:    statuses,
		CreatedAt:   project.CreatedAt,
	}, nil
}

func (s *ProjectService) Update(projectID, userID uint, req UpdateProjectRequest) (*ProjectDetail, error) {
	workspaceID, err := RequireProjectRole(s.db, projectID, userID, models.RoleAdmin)
	if err != nil {
		return nil, err
	}

	var project models.Project
	if err := s.db.First(&project, projectID).Error; err != nil {
		return nil, utils.NotFound("project not found")
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, utils.BadRequest("project name cannot be empty")
		}
		project.Name = name
	}
	if req.Description != nil {
		project.Description = req.Description
	}
	if req.IsPrivate != nil {
		project.IsPrivate = *req.IsPrivate
	}

	if err := s.db.Save(&project).Error; err != nil {
		return nil, utils.Internal()
	}

	LogActivity(s.db, workspaceID, ptr(projectID), nil, ptr(userID), "project.updated", map[string]any{
		"project_id": projectID,
		"name":       project.Name,
	})

	s.hub.BroadcastToProject(projectID, realtime.Message{
		Type:      "project.updated",
		ProjectID: projectID,
		Data: map[string]any{
			"id":          project.ID,
			"name":        project.Name,
			"description": project.Description,
		},
	})
	return s.Get(projectID, userID)
}

func (s *ProjectService) Delete(projectID, userID uint) error {
	workspaceID, err := RequireProjectRole(s.db, projectID, userID, models.RoleAdmin)
	if err != nil {
		return err
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		var taskIDs []uint
		if err := tx.Model(&models.Task{}).Where("project_id = ?", projectID).Pluck("id", &taskIDs).Error; err != nil {
			return err
		}
		if len(taskIDs) > 0 {
			if err := tx.Where("task_id IN ?", taskIDs).Delete(&models.Comment{}).Error; err != nil {
				return err
			}
			if err := tx.Where("task_id IN ?", taskIDs).Delete(&models.TaskAssignee{}).Error; err != nil {
				return err
			}
			if err := tx.Where("task_id IN ?", taskIDs).Delete(&models.TaskDocState{}).Error; err != nil {
				return err
			}
			if err := tx.Where("task_id IN ?", taskIDs).Delete(&models.Notification{}).Error; err != nil {
				return err
			}
			if err := tx.Where("project_id = ?", projectID).Delete(&models.Task{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("project_id = ?", projectID).Delete(&models.ProjectStatus{}).Error; err != nil {
			return err
		}
		if err := tx.Where("project_id = ?", projectID).Delete(&models.Notification{}).Error; err != nil {
			return err
		}
		if err := tx.Where("project_id = ?", projectID).Delete(&models.ActivityLog{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.Project{}, projectID).Error
	})
	if err != nil {
		if ae, ok := err.(*utils.AppError); ok {
			return ae
		}
		return utils.Internal()
	}

	LogActivity(s.db, workspaceID, nil, nil, ptr(userID), "project.deleted", map[string]any{
		"project_id": projectID,
	})

	s.hub.BroadcastToProject(projectID, realtime.Message{
		Type:      "project.deleted",
		ProjectID: projectID,
		Data:      map[string]any{"id": projectID},
	})
	s.hub.CloseProject(projectID)
	return nil
}

func (s *ProjectService) listStatuses(projectID uint) ([]StatusResponse, error) {
	var statuses []models.ProjectStatus
	if err := s.db.Where("project_id = ?", projectID).
		Order("position ASC, id ASC").
		Find(&statuses).Error; err != nil {
		return nil, utils.Internal()
	}
	out := make([]StatusResponse, 0, len(statuses))
	for _, st := range statuses {
		out = append(out, toStatusResponse(st))
	}
	return out, nil
}

func toStatusResponse(st models.ProjectStatus) StatusResponse {
	return StatusResponse{
		ID:        st.ID,
		ProjectID: st.ProjectID,
		Name:      st.Name,
		Color:     st.Color,
		Position:  st.Position,
		CreatedAt: st.CreatedAt,
	}
}
