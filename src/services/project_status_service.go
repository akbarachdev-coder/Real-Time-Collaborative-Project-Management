package services

import (
	"strings"

	"collabpm/models"
	"collabpm/realtime"
	"collabpm/utils"

	"gorm.io/gorm"
)

type ProjectStatusService struct {
	db  *gorm.DB
	hub *realtime.Hub
}

func NewProjectStatusService(db *gorm.DB, hub *realtime.Hub) *ProjectStatusService {
	return &ProjectStatusService{db: db, hub: hub}
}

type CreateStatusRequest struct {
	Name     string   `json:"name" binding:"required,max=100"`
	Color    string   `json:"color" binding:"omitempty,max=7"`
	Position *float64 `json:"position"`
}

type UpdateStatusRequest struct {
	Name     *string  `json:"name" binding:"omitempty,min=1,max=100"`
	Color    *string  `json:"color" binding:"omitempty,max=7"`
	Position *float64 `json:"position"`
}

type ReorderStatusItem struct {
	ID       uint     `json:"id" binding:"required"`
	Position *float64 `json:"position" binding:"required"`
}

type ReorderStatusesRequest struct {
	Items []ReorderStatusItem `json:"items" binding:"required,min=1"`
}

func (s *ProjectStatusService) List(projectID, userID uint) ([]StatusResponse, error) {
	if _, err := workspaceIDForProject(s.db, projectID, userID); err != nil {
		return nil, err
	}
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

func (s *ProjectStatusService) Create(projectID, userID uint, req CreateStatusRequest) (*StatusResponse, error) {
	workspaceID, err := RequireProjectRole(s.db, projectID, userID, models.RoleAdmin)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, utils.BadRequest("status name cannot be empty")
	}

	var count int64
	if err := s.db.Model(&models.ProjectStatus{}).
		Where("project_id = ? AND name = ?", projectID, name).
		Count(&count).Error; err != nil {
		return nil, utils.Internal()
	}
	if count > 0 {
		return nil, utils.Conflict("status name already exists in this project")
	}

	position := float64(0)
	if req.Position != nil {
		position = *req.Position
	} else {
		var maxPos float64
		if err := s.db.Model(&models.ProjectStatus{}).
			Where("project_id = ?", projectID).
			Select("COALESCE(MAX(position), 0)").
			Scan(&maxPos).Error; err != nil {
			return nil, utils.Internal()
		}
		position = maxPos + 10
	}

	color := req.Color
	if color == "" {
		color = "#888888"
	}

	status := models.ProjectStatus{
		ProjectID: projectID,
		Name:      name,
		Color:     color,
		Position:  position,
	}
	if err := s.db.Create(&status).Error; err != nil {
		return nil, utils.Internal()
	}

	LogActivity(s.db, workspaceID, ptr(projectID), nil, ptr(userID), "status.created", map[string]any{
		"status_id": status.ID,
		"name":      status.Name,
	})
	s.broadcast(projectID, "status.created", status)

	resp := toStatusResponse(status)
	return &resp, nil
}

func (s *ProjectStatusService) Update(projectID, statusID, userID uint, req UpdateStatusRequest) (*StatusResponse, error) {
	workspaceID, err := RequireProjectRole(s.db, projectID, userID, models.RoleAdmin)
	if err != nil {
		return nil, err
	}

	var status models.ProjectStatus
	if err := s.db.Where("id = ? AND project_id = ?", statusID, projectID).First(&status).Error; err != nil {
		return nil, utils.NotFound("status not found")
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, utils.BadRequest("status name cannot be empty")
		}
		var count int64
		if err := s.db.Model(&models.ProjectStatus{}).
			Where("project_id = ? AND name = ? AND id != ?", projectID, name, statusID).
			Count(&count).Error; err != nil {
			return nil, utils.Internal()
		}
		if count > 0 {
			return nil, utils.Conflict("status name already exists in this project")
		}
		status.Name = name
	}
	if req.Color != nil {
		status.Color = *req.Color
	}
	if req.Position != nil {
		status.Position = *req.Position
	}

	if err := s.db.Save(&status).Error; err != nil {
		return nil, utils.Internal()
	}

	LogActivity(s.db, workspaceID, ptr(projectID), nil, ptr(userID), "status.updated", map[string]any{
		"status_id": status.ID,
		"name":      status.Name,
	})
	s.broadcast(projectID, "status.updated", status)

	resp := toStatusResponse(status)
	return &resp, nil
}

func (s *ProjectStatusService) Delete(projectID, statusID, userID uint) error {
	workspaceID, err := RequireProjectRole(s.db, projectID, userID, models.RoleAdmin)
	if err != nil {
		return err
	}

	var status models.ProjectStatus
	if err := s.db.Where("id = ? AND project_id = ?", statusID, projectID).First(&status).Error; err != nil {
		return utils.NotFound("status not found")
	}

	var count int64
	if err := s.db.Model(&models.Task{}).
		Where("status_id = ?", statusID).
		Count(&count).Error; err != nil {
		return utils.Internal()
	}
	if count > 0 {
		return utils.Conflict("cannot delete a status that still has tasks")
	}

	var total int64
	if err := s.db.Model(&models.ProjectStatus{}).
		Where("project_id = ?", projectID).
		Count(&total).Error; err != nil {
		return utils.Internal()
	}
	if total <= 1 {
		return utils.BadRequest("project must keep at least one status")
	}

	if err := s.db.Delete(&status).Error; err != nil {
		return utils.Internal()
	}

	LogActivity(s.db, workspaceID, ptr(projectID), nil, ptr(userID), "status.deleted", map[string]any{
		"status_id": statusID,
		"name":      status.Name,
	})
	s.hub.BroadcastToProject(projectID, realtime.Message{
		Type:      "status.deleted",
		ProjectID: projectID,
		Data:      map[string]any{"id": statusID},
	})
	return nil
}

func (s *ProjectStatusService) Reorder(projectID, userID uint, req ReorderStatusesRequest) ([]StatusResponse, error) {
	workspaceID, err := RequireProjectRole(s.db, projectID, userID, models.RoleAdmin)
	if err != nil {
		return nil, err
	}

	ids := make([]uint, 0, len(req.Items))
	for _, item := range req.Items {
		ids = append(ids, item.ID)
	}

	var statuses []models.ProjectStatus
	if err := s.db.Where("project_id = ? AND id IN ?", projectID, ids).Find(&statuses).Error; err != nil {
		return nil, utils.Internal()
	}
	if len(statuses) != len(ids) {
		return nil, utils.NotFound("one or more statuses not found in this project")
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		for _, item := range req.Items {
			if err := tx.Model(&models.ProjectStatus{}).
				Where("id = ? AND project_id = ?", item.ID, projectID).
				Update("position", *item.Position).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, utils.Internal()
	}

	LogActivity(s.db, workspaceID, ptr(projectID), nil, ptr(userID), "status.reordered", map[string]any{
		"count": len(req.Items),
	})
	return s.List(projectID, userID)
}

func (s *ProjectStatusService) broadcast(projectID uint, event string, status models.ProjectStatus) {
	s.hub.BroadcastToProject(projectID, realtime.Message{
		Type:      event,
		ProjectID: projectID,
		Data:      toStatusResponse(status),
	})
}
