package services

import (
	"collabpm/models"
	"collabpm/utils"

	"gorm.io/gorm"
)

// WorkspaceMembership returns the caller's role in a workspace, or
// NotFound if the user is not a member (avoids leaking existence).
func WorkspaceMembership(db *gorm.DB, workspaceID, userID uint) (string, error) {
	var member models.WorkspaceMember
	err := db.Where("workspace_id = ? AND user_id = ?", workspaceID, userID).First(&member).Error
	if err == gorm.ErrRecordNotFound {
		return "", utils.NotFound("workspace not found")
	}
	if err != nil {
		return "", utils.Internal()
	}
	return member.Role, nil
}

func RequireWorkspaceMember(db *gorm.DB, workspaceID, userID uint) error {
	_, err := WorkspaceMembership(db, workspaceID, userID)
	return err
}

// RequireWorkspaceRole enforces a minimum role (GUEST < MEMBER < ADMIN < OWNER).
func RequireWorkspaceRole(db *gorm.DB, workspaceID, userID uint, minRole string) error {
	role, err := WorkspaceMembership(db, workspaceID, userID)
	if err != nil {
		return err
	}
	if !models.RoleAtLeast(role, minRole) {
		return utils.Forbidden("insufficient workspace role")
	}
	return nil
}

// workspaceIDForProject resolves a project to its workspace and verifies
// the user is a member. Returns workspaceID for activity logging.
func workspaceIDForProject(db *gorm.DB, projectID, userID uint) (uint, error) {
	var project models.Project
	if err := db.First(&project, projectID).Error; err != nil {
		return 0, utils.NotFound("project not found")
	}
	if err := RequireWorkspaceMember(db, project.WorkspaceID, userID); err != nil {
		return 0, err
	}
	return project.WorkspaceID, nil
}

// RequireProjectRole resolves project → workspace and enforces a min role.
func RequireProjectRole(db *gorm.DB, projectID, userID uint, minRole string) (uint, error) {
	var project models.Project
	if err := db.First(&project, projectID).Error; err != nil {
		return 0, utils.NotFound("project not found")
	}
	if err := RequireWorkspaceRole(db, project.WorkspaceID, userID, minRole); err != nil {
		return 0, err
	}
	return project.WorkspaceID, nil
}

// IsProjectMember reports whether the user can access a project (WS subscribe).
func IsProjectMember(db *gorm.DB, projectID, userID uint) (bool, error) {
	var project models.Project
	if err := db.First(&project, projectID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, err
	}
	var count int64
	if err := db.Model(&models.WorkspaceMember{}).
		Where("workspace_id = ? AND user_id = ?", project.WorkspaceID, userID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
