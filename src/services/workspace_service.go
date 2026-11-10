package services

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"collabpm/models"
	"collabpm/utils"

	"gorm.io/gorm"
)

type WorkspaceService struct {
	db *gorm.DB
}

func NewWorkspaceService(db *gorm.DB) *WorkspaceService {
	return &WorkspaceService{db: db}
}

type CreateWorkspaceRequest struct {
	Name string `json:"name" binding:"required,max=255"`
}

type UpdateWorkspaceRequest struct {
	Name *string `json:"name" binding:"omitempty,min=1,max=255"`
}

type AddWorkspaceMemberRequest struct {
	UserID *uint  `json:"user_id"`
	Email  string `json:"email" binding:"omitempty,email,max=191"`
	Role   string `json:"role" binding:"omitempty,oneof=ADMIN MEMBER GUEST"`
}

type UpdateWorkspaceMemberRequest struct {
	Role string `json:"role" binding:"required,oneof=ADMIN MEMBER GUEST"`
}

type WorkspaceSummary struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	OwnerID   uint      `json:"owner_id"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

type WorkspaceDetail struct {
	ID        uint                      `json:"id"`
	Name      string                    `json:"name"`
	Slug      string                    `json:"slug"`
	OwnerID   uint                      `json:"owner_id"`
	Role      string                    `json:"role"`
	Members   []WorkspaceMemberResponse `json:"members"`
	CreatedAt time.Time                 `json:"created_at"`
}

type WorkspaceMemberResponse struct {
	UserID   uint         `json:"user_id"`
	Role     string       `json:"role"`
	JoinedAt time.Time    `json:"joined_at"`
	User     UserResponse `json:"user"`
}

func (s *WorkspaceService) List(userID uint) ([]WorkspaceSummary, error) {
	var rows []struct {
		ID        uint
		Name      string
		Slug      string
		OwnerID   uint
		Role      string
		CreatedAt time.Time
	}
	err := s.db.Table("workspaces").
		Select("workspaces.id, workspaces.name, workspaces.slug, workspaces.owner_id, workspace_members.role, workspaces.created_at").
		Joins("JOIN workspace_members ON workspace_members.workspace_id = workspaces.id").
		Where("workspace_members.user_id = ?", userID).
		Order("workspaces.created_at DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, utils.Internal()
	}
	out := make([]WorkspaceSummary, 0, len(rows))
	for _, r := range rows {
		out = append(out, WorkspaceSummary{
			ID:        r.ID,
			Name:      r.Name,
			Slug:      r.Slug,
			OwnerID:   r.OwnerID,
			Role:      r.Role,
			CreatedAt: r.CreatedAt,
		})
	}
	return out, nil
}

func (s *WorkspaceService) Create(userID uint, req CreateWorkspaceRequest) (*WorkspaceDetail, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, utils.BadRequest("workspace name cannot be empty")
	}

	slug, err := s.uniqueSlug(name)
	if err != nil {
		return nil, err
	}

	var ws models.Workspace
	err = s.db.Transaction(func(tx *gorm.DB) error {
		ws = models.Workspace{
			Name:    name,
			Slug:    slug,
			OwnerID: userID,
		}
		if err := tx.Create(&ws).Error; err != nil {
			return err
		}
		return tx.Create(&models.WorkspaceMember{
			WorkspaceID: ws.ID,
			UserID:      userID,
			Role:        models.RoleOwner,
		}).Error
	})
	if err != nil {
		return nil, utils.Internal()
	}
	return s.Get(ws.ID, userID)
}

func (s *WorkspaceService) Get(workspaceID, userID uint) (*WorkspaceDetail, error) {
	role, err := WorkspaceMembership(s.db, workspaceID, userID)
	if err != nil {
		return nil, err
	}

	var ws models.Workspace
	if err := s.db.First(&ws, workspaceID).Error; err != nil {
		return nil, utils.NotFound("workspace not found")
	}

	members, err := s.listMembers(workspaceID)
	if err != nil {
		return nil, err
	}

	return &WorkspaceDetail{
		ID:        ws.ID,
		Name:      ws.Name,
		Slug:      ws.Slug,
		OwnerID:   ws.OwnerID,
		Role:      role,
		Members:   members,
		CreatedAt: ws.CreatedAt,
	}, nil
}

func (s *WorkspaceService) Update(workspaceID, userID uint, req UpdateWorkspaceRequest) (*WorkspaceDetail, error) {
	if err := RequireWorkspaceRole(s.db, workspaceID, userID, models.RoleAdmin); err != nil {
		return nil, err
	}

	var ws models.Workspace
	if err := s.db.First(&ws, workspaceID).Error; err != nil {
		return nil, utils.NotFound("workspace not found")
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, utils.BadRequest("workspace name cannot be empty")
		}
		ws.Name = name
	}

	if err := s.db.Save(&ws).Error; err != nil {
		return nil, utils.Internal()
	}
	return s.Get(workspaceID, userID)
}

func (s *WorkspaceService) Delete(workspaceID, userID uint) error {
	if err := RequireWorkspaceRole(s.db, workspaceID, userID, models.RoleOwner); err != nil {
		return err
	}

	var ws models.Workspace
	if err := s.db.First(&ws, workspaceID).Error; err != nil {
		return utils.NotFound("workspace not found")
	}
	if ws.OwnerID != userID {
		return utils.Forbidden("only the workspace owner can delete this workspace")
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		var projectIDs []uint
		if err := tx.Model(&models.Project{}).Where("workspace_id = ?", workspaceID).Pluck("id", &projectIDs).Error; err != nil {
			return err
		}
		if len(projectIDs) > 0 {
			var taskIDs []uint
			if err := tx.Model(&models.Task{}).Where("project_id IN ?", projectIDs).Pluck("id", &taskIDs).Error; err != nil {
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
				if err := tx.Where("project_id IN ?", projectIDs).Delete(&models.Task{}).Error; err != nil {
					return err
				}
			}
			if err := tx.Where("project_id IN ?", projectIDs).Delete(&models.ProjectStatus{}).Error; err != nil {
				return err
			}
			if err := tx.Where("project_id IN ?", projectIDs).Delete(&models.Notification{}).Error; err != nil {
				return err
			}
			if err := tx.Where("project_id IN ?", projectIDs).Delete(&models.ActivityLog{}).Error; err != nil {
				return err
			}
			if err := tx.Where("id IN ?", projectIDs).Delete(&models.Project{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("workspace_id = ?", workspaceID).Delete(&models.ActivityLog{}).Error; err != nil {
			return err
		}
		if err := tx.Where("workspace_id = ?", workspaceID).Delete(&models.WorkspaceMember{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.Workspace{}, workspaceID).Error
	})
	if err != nil {
		return utils.Internal()
	}
	return nil
}

func (s *WorkspaceService) ListMembers(workspaceID, userID uint) ([]WorkspaceMemberResponse, error) {
	if err := RequireWorkspaceMember(s.db, workspaceID, userID); err != nil {
		return nil, err
	}
	return s.listMembers(workspaceID)
}

func (s *WorkspaceService) AddMember(workspaceID, actorID uint, req AddWorkspaceMemberRequest) ([]WorkspaceMemberResponse, error) {
	if err := RequireWorkspaceRole(s.db, workspaceID, actorID, models.RoleAdmin); err != nil {
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

	role := req.Role
	if role == "" {
		role = models.RoleMember
	}
	if !models.IsValidRole(role) || role == models.RoleOwner {
		return nil, utils.BadRequest("invalid role")
	}

	var count int64
	if err := s.db.Model(&models.WorkspaceMember{}).
		Where("workspace_id = ? AND user_id = ?", workspaceID, target.ID).
		Count(&count).Error; err != nil {
		return nil, utils.Internal()
	}
	if count > 0 {
		return nil, utils.Conflict("user is already a member of this workspace")
	}

	if err := s.db.Create(&models.WorkspaceMember{
		WorkspaceID: workspaceID,
		UserID:      target.ID,
		Role:        role,
	}).Error; err != nil {
		return nil, utils.Internal()
	}

	LogActivity(s.db, workspaceID, nil, nil, ptr(actorID), "member.added", map[string]any{
		"user_id": target.ID,
		"role":    role,
	})

	return s.listMembers(workspaceID)
}

func (s *WorkspaceService) UpdateMemberRole(workspaceID, actorID, targetUserID uint, req UpdateWorkspaceMemberRequest) ([]WorkspaceMemberResponse, error) {
	if err := RequireWorkspaceRole(s.db, workspaceID, actorID, models.RoleAdmin); err != nil {
		return nil, err
	}

	var member models.WorkspaceMember
	err := s.db.Where("workspace_id = ? AND user_id = ?", workspaceID, targetUserID).First(&member).Error
	if err == gorm.ErrRecordNotFound {
		return nil, utils.NotFound("member not found")
	}
	if err != nil {
		return nil, utils.Internal()
	}
	if member.Role == models.RoleOwner {
		return nil, utils.BadRequest("the workspace owner role cannot be changed")
	}
	if req.Role == models.RoleOwner {
		return nil, utils.BadRequest("only OWNER can be set at creation")
	}
	if member.Role == req.Role {
		return s.listMembers(workspaceID)
	}

	if err := s.db.Model(&member).Update("role", req.Role).Error; err != nil {
		return nil, utils.Internal()
	}

	LogActivity(s.db, workspaceID, nil, nil, ptr(actorID), "member.role_changed", map[string]any{
		"user_id": targetUserID,
		"role":    req.Role,
	})
	return s.listMembers(workspaceID)
}

func (s *WorkspaceService) RemoveMember(workspaceID, actorID, targetUserID uint) error {
	isSelf := actorID == targetUserID
	if !isSelf {
		if err := RequireWorkspaceRole(s.db, workspaceID, actorID, models.RoleAdmin); err != nil {
			return err
		}
	} else if err := RequireWorkspaceMember(s.db, workspaceID, actorID); err != nil {
		return err
	}

	var member models.WorkspaceMember
	err := s.db.Where("workspace_id = ? AND user_id = ?", workspaceID, targetUserID).First(&member).Error
	if err == gorm.ErrRecordNotFound {
		return utils.NotFound("member not found")
	}
	if err != nil {
		return utils.Internal()
	}
	if member.Role == models.RoleOwner {
		return utils.BadRequest("the workspace owner cannot be removed")
	}
	if !isSelf && member.Role == models.RoleAdmin && actorID != targetUserID {
		// Admins may remove members/guests but not other admins unless owner.
		var actor models.WorkspaceMember
		if err := s.db.Where("workspace_id = ? AND user_id = ?", workspaceID, actorID).First(&actor).Error; err != nil {
			return utils.Internal()
		}
		if actor.Role != models.RoleOwner {
			return utils.Forbidden("only the owner can remove an admin")
		}
	}

	if err := s.db.Delete(&member).Error; err != nil {
		return utils.Internal()
	}

	LogActivity(s.db, workspaceID, nil, nil, ptr(actorID), "member.removed", map[string]any{
		"user_id": targetUserID,
	})
	return nil
}

func (s *WorkspaceService) listMembers(workspaceID uint) ([]WorkspaceMemberResponse, error) {
	var rows []struct {
		UserID   uint
		Role     string
		JoinedAt time.Time
		Name     string
		Email    string
		Avatar   string
	}
	err := s.db.Table("workspace_members").
		Select("workspace_members.user_id, workspace_members.role, workspace_members.joined_at, users.name, users.email, users.avatar").
		Joins("JOIN users ON users.id = workspace_members.user_id").
		Where("workspace_members.workspace_id = ?", workspaceID).
		Order("workspace_members.role DESC, users.name ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, utils.Internal()
	}

	members := make([]WorkspaceMemberResponse, 0, len(rows))
	for _, r := range rows {
		members = append(members, WorkspaceMemberResponse{
			UserID:   r.UserID,
			Role:     r.Role,
			JoinedAt: r.JoinedAt,
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

var slugSanitizer = regexp.MustCompile(`[^a-z0-9]+`)

func (s *WorkspaceService) uniqueSlug(name string) (string, error) {
	base := slugSanitizer.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), "-")
	base = strings.Trim(base, "-")
	if base == "" {
		base = "workspace"
	}
	if len(base) > 40 {
		base = base[:40]
	}

	slug := base
	for i := 0; i < 50; i++ {
		var count int64
		if err := s.db.Model(&models.Workspace{}).Where("slug = ?", slug).Count(&count).Error; err != nil {
			return "", utils.Internal()
		}
		if count == 0 {
			return slug, nil
		}
		slug = fmt.Sprintf("%s-%d", base, i+2)
	}
	return "", utils.Conflict("could not generate unique slug")
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
