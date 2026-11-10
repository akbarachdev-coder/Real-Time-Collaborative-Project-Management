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

type ProjectService struct {
	db  *gorm.DB
	hub *realtime.Hub
}

func NewProjectService(db *gorm.DB, hub *realtime.Hub) *ProjectService {
	return &ProjectService{db: db, hub: hub}
}

type CreateProjectRequest struct {
	Name        string `json:"name" binding:"required,max=150"`
	Description string `json:"description" binding:"max=5000"`
}

type UpdateProjectRequest struct {
	Name        *string `json:"name" binding:"omitempty,min=1,max=150"`
	Description *string `json:"description" binding:"omitempty,max=5000"`
}

type AddMemberRequest struct {
	UserID *uint  `json:"user_id"`
	Email  string `json:"email" binding:"omitempty,email,max=191"`
}

type ProjectSummary struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	OwnerID     uint      `json:"owner_id"`
	Role        string    `json:"role"`
	MemberCount int64     `json:"member_count"`
	CreatedAt   time.Time `json:"created_at"`
}

type MemberResponse struct {
	UserID uint         `json:"user_id"`
	Role   string       `json:"role"`
	User   UserResponse `json:"user"`
}

type ProjectDetail struct {
	ID          uint             `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	OwnerID     uint             `json:"owner_id"`
	Role        string           `json:"role"`
	Members     []MemberResponse `json:"members"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

func (s *ProjectService) List(userID uint) ([]ProjectSummary, error) {
	var out []ProjectSummary
	err := s.db.Table("projects").
		Select("projects.*, project_members.role AS role, (SELECT COUNT(*) FROM project_members pm2 WHERE pm2.project_id = projects.id) AS member_count").
		Joins("JOIN project_members ON project_members.project_id = projects.id").
		Where("project_members.user_id = ?", userID).
		Order("projects.created_at DESC").
		Scan(&out).Error
	if err != nil {
		return nil, utils.Internal()
	}
	if out == nil {
		out = []ProjectSummary{}
	}
	return out, nil
}

func (s *ProjectService) Create(userID uint, req CreateProjectRequest) (*ProjectDetail, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, utils.BadRequest("project name cannot be empty")
	}

	var project models.Project
	err := s.db.Transaction(func(tx *gorm.DB) error {
		project = models.Project{
			Name:        name,
			Description: req.Description,
			OwnerID:     userID,
		}
		if err := tx.Create(&project).Error; err != nil {
			return err
		}
		return tx.Create(&models.ProjectMember{
			ProjectID: project.ID,
			UserID:    userID,
			Role:      "owner",
		}).Error
	})
	if err != nil {
		return nil, utils.Internal()
	}
	return s.Get(project.ID, userID)
}

func (s *ProjectService) Get(projectID, userID uint) (*ProjectDetail, error) {
	if err := RequireProjectMember(s.db, projectID, userID); err != nil {
		return nil, err
	}

	var project models.Project
	if err := s.db.First(&project, projectID).Error; err != nil {
		return nil, utils.NotFound("project not found")
	}

	members, err := s.listMembers(projectID)
	if err != nil {
		return nil, err
	}

	role := ""
	for _, m := range members {
		if m.UserID == userID {
			role = m.Role
			break
		}
	}

	return &ProjectDetail{
		ID:          project.ID,
		Name:        project.Name,
		Description: project.Description,
		OwnerID:     project.OwnerID,
		Role:        role,
		Members:     members,
		CreatedAt:   project.CreatedAt,
		UpdatedAt:   project.UpdatedAt,
	}, nil
}

func (s *ProjectService) Update(projectID, userID uint, req UpdateProjectRequest) (*ProjectDetail, error) {
	if err := RequireProjectOwner(s.db, projectID, userID); err != nil {
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
		project.Description = *req.Description
	}

	if err := s.db.Save(&project).Error; err != nil {
		return nil, utils.Internal()
	}

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
	if err := RequireProjectOwner(s.db, projectID, userID); err != nil {
		return err
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		var taskIDs []uint
		if err := tx.Model(&models.Task{}).Where("project_id = ?", projectID).Pluck("id", &taskIDs).Error; err != nil {
			return err
		}
		if len(taskIDs) > 0 {
			if err := tx.Where("task_id IN ?", taskIDs).Delete(&models.Comment{}).Error; err != nil {
				return err
			}
			if err := tx.Where("task_id IN ?", taskIDs).Delete(&models.Notification{}).Error; err != nil {
				return err
			}
			if err := tx.Where("project_id = ?", projectID).Delete(&models.Task{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("project_id = ?", projectID).Delete(&models.Notification{}).Error; err != nil {
			return err
		}
		if err := tx.Where("project_id = ?", projectID).Delete(&models.ProjectMember{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.Project{}, projectID).Error
	})
	if err != nil {
		return utils.Internal()
	}

	s.hub.BroadcastToProject(projectID, realtime.Message{
		Type:      "project.deleted",
		ProjectID: projectID,
		Data:      map[string]any{"id": projectID},
	})
	s.hub.CloseProject(projectID)
	return nil
}

func (s *ProjectService) ListMembers(projectID, userID uint) ([]MemberResponse, error) {
	if err := RequireProjectMember(s.db, projectID, userID); err != nil {
		return nil, err
	}
	return s.listMembers(projectID)
}

func (s *ProjectService) AddMember(projectID, actorID uint, req AddMemberRequest) ([]MemberResponse, error) {
	if err := RequireProjectMember(s.db, projectID, actorID); err != nil {
		return nil, err
	}

	var target models.User
	var err error
	switch {
	case req.UserID != nil:
		err = s.db.First(&target, *req.UserID).Error
	case req.Email != "":
		err = s.db.Where("email = ?", normalizeEmail(req.Email)).First(&target).Error
	default:
		return nil, utils.BadRequest("either user_id or email is required")
	}
	if err == gorm.ErrRecordNotFound {
		return nil, utils.NotFound("user not found")
	}
	if err != nil {
		return nil, utils.Internal()
	}

	var count int64
	if err := s.db.Model(&models.ProjectMember{}).
		Where("project_id = ? AND user_id = ?", projectID, target.ID).
		Count(&count).Error; err != nil {
		return nil, utils.Internal()
	}
	if count > 0 {
		return nil, utils.Conflict("user is already a member of this project")
	}

	if err := s.db.Create(&models.ProjectMember{
		ProjectID: projectID,
		UserID:    target.ID,
		Role:      "member",
	}).Error; err != nil {
		return nil, utils.Internal()
	}

	var project models.Project
	if err := s.db.First(&project, projectID).Error; err != nil {
		return nil, utils.Internal()
	}
	var actor models.User
	if err := s.db.First(&actor, actorID).Error; err != nil {
		return nil, utils.Internal()
	}

	member := MemberResponse{
		UserID: target.ID,
		Role:   "member",
		User:   toUserResponse(target),
	}
	s.hub.BroadcastToProject(projectID, realtime.Message{
		Type:      "member.added",
		ProjectID: projectID,
		Data:      member,
	})
	createNotification(s.db, s.hub, target.ID, models.NotificationMemberAdded,
		fmt.Sprintf("%s added you to project %q", actor.Name, project.Name),
		ptrUint(projectID), nil)

	members, err := s.listMembers(projectID)
	if err != nil {
		return nil, err
	}
	return members, nil
}

func (s *ProjectService) RemoveMember(projectID, actorID, targetUserID uint) error {
	isSelf := actorID == targetUserID
	if !isSelf {
		if err := RequireProjectOwner(s.db, projectID, actorID); err != nil {
			return err
		}
	}

	var member models.ProjectMember
	err := s.db.Where("project_id = ? AND user_id = ?", projectID, targetUserID).First(&member).Error
	if err == gorm.ErrRecordNotFound {
		return utils.NotFound("member not found")
	}
	if err != nil {
		return utils.Internal()
	}
	if member.Role == "owner" {
		return utils.BadRequest("the project owner cannot be removed")
	}

	if err := s.db.Delete(&member).Error; err != nil {
		return utils.Internal()
	}

	s.hub.BroadcastToProject(projectID, realtime.Message{
		Type:      "member.removed",
		ProjectID: projectID,
		Data:      map[string]any{"user_id": targetUserID},
	})
	s.hub.RemoveUserFromProject(projectID, targetUserID)

	if !isSelf {
		var project models.Project
		var actor models.User
		if err := s.db.First(&project, projectID).Error; err == nil &&
			s.db.First(&actor, actorID).Error == nil {
			createNotification(s.db, s.hub, targetUserID, models.NotificationMemberRemoved,
				fmt.Sprintf("%s removed you from project %q", actor.Name, project.Name),
				ptrUint(projectID), nil)
		}
	}
	return nil
}

func (s *ProjectService) listMembers(projectID uint) ([]MemberResponse, error) {
	var rows []struct {
		UserID uint
		Role   string
		Name   string
		Email  string
		Avatar string
	}
	err := s.db.Table("project_members").
		Select("project_members.user_id, project_members.role, users.name, users.email, users.avatar").
		Joins("JOIN users ON users.id = project_members.user_id").
		Where("project_members.project_id = ?", projectID).
		Order("project_members.role DESC, users.name ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, utils.Internal()
	}

	members := make([]MemberResponse, 0, len(rows))
	for _, r := range rows {
		members = append(members, MemberResponse{
			UserID: r.UserID,
			Role:   r.Role,
			User: UserResponse{
				ID:     r.UserID,
				Name:   r.Name,
				Email:  r.Email,
				Avatar: r.Avatar,
			},
		})
	}
	return members, nil
}

func ptrUint(v uint) *uint {
	return &v
}
